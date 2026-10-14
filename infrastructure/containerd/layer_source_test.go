package containerd_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/leases"
	"github.com/containerd/platforms"
	"github.com/golang/mock/gomock"
	g "github.com/onsi/gomega"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/liquidmetal-dev/flintlock/infrastructure/containerd"
	"github.com/liquidmetal-dev/flintlock/infrastructure/mock"
	"github.com/liquidmetal-dev/flintlock/infrastructure/volume/overlay"
)

// memReaderAt is a content.ReaderAt over a byte slice.
type memReaderAt struct {
	*bytes.Reader
}

func (r memReaderAt) Size() int64  { return int64(r.Len()) }
func (r memReaderAt) Close() error { return nil }

// blobStore maps digests to blobs and serves them through a mock content
// store.
type blobStore struct {
	blobs map[digest.Digest][]byte
}

func (b *blobStore) add(mediaType string, data []byte) ocispec.Descriptor {
	d := digest.FromBytes(data)
	b.blobs[d] = data

	return ocispec.Descriptor{MediaType: mediaType, Digest: d, Size: int64(len(data))}
}

func (b *blobStore) readerAt(_ context.Context, desc ocispec.Descriptor) (content.ReaderAt, error) {
	data, ok := b.blobs[desc.Digest]
	if !ok {
		return nil, fmt.Errorf("blob %s not found", desc.Digest)
	}

	return memReaderAt{bytes.NewReader(data)}, nil
}

func layerTar(t *testing.T, name, body string) []byte {
	t.Helper()

	var buf bytes.Buffer

	tw := tar.NewWriter(&buf)
	hdr := &tar.Header{Typeflag: tar.TypeReg, Name: name, Mode: 0o644, Size: int64(len(body))}

	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}

	if _, err := io.WriteString(tw, body); err != nil {
		t.Fatal(err)
	}

	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	return buf.Bytes()
}

func gzipBytes(t *testing.T, data []byte) []byte {
	t.Helper()

	var buf bytes.Buffer

	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(data); err != nil {
		t.Fatal(err)
	}

	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	return buf.Bytes()
}

// testImage is a two-layer image in a blob store: the first layer is
// gzipped, the second is not.
type fakeImage struct {
	store    *blobStore
	manifest ocispec.Descriptor
	tars     [][]byte
	diffIDs  []digest.Digest
}

func newTestImage(t *testing.T, mediaTypes []string, diffIDs []digest.Digest) fakeImage {
	t.Helper()

	store := &blobStore{blobs: map[digest.Digest][]byte{}}

	tars := [][]byte{
		layerTar(t, "etc/a", "first"),
		layerTar(t, "etc/b", "second"),
	}

	if diffIDs == nil {
		for _, l := range tars {
			diffIDs = append(diffIDs, digest.FromBytes(l))
		}
	}

	cfg, err := json.Marshal(ocispec.Image{RootFS: ocispec.RootFS{Type: "layers", DiffIDs: diffIDs}})
	if err != nil {
		t.Fatal(err)
	}

	manifest := ocispec.Manifest{
		MediaType: ocispec.MediaTypeImageManifest,
		Config:    store.add(ocispec.MediaTypeImageConfig, cfg),
		Layers: []ocispec.Descriptor{
			store.add(mediaTypes[0], gzipBytes(t, tars[0])),
			store.add(mediaTypes[1], tars[1]),
		},
	}

	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}

	return fakeImage{
		store:    store,
		manifest: store.add(ocispec.MediaTypeImageManifest, manifestJSON),
		tars:     tars,
		diffIDs:  diffIDs,
	}
}

// layerSourceWithMocks wires a LayerSource to mocks that pull the image
// under the owner's lease and serve the blobs of img.
func layerSourceWithMocks(t *testing.T, img fakeImage) overlay.LayerSource {
	t.Helper()

	mockCtrl := gomock.NewController(t)
	client := mock.NewMockClient(mockCtrl)
	leasesManager := mock.NewMockManager(mockCtrl)
	image := mock.NewMockImage(mockCtrl)
	store := mock.NewMockStore(mockCtrl)

	leasesManager.EXPECT().
		List(gomock.Any(), "id==flintlock/"+testOwner).
		Return([]leases.Lease{{ID: "flintlock/" + testOwner}}, nil)
	client.EXPECT().LeasesService().Return(leasesManager)
	client.EXPECT().Pull(gomock.Any(), testImage).Return(image, nil)

	image.EXPECT().RootFS(gomock.Any()).Return(img.diffIDs, nil)
	image.EXPECT().ContentStore().Return(store)
	image.EXPECT().Target().Return(img.manifest)
	image.EXPECT().Platform().Return(platforms.Default())
	store.EXPECT().ReaderAt(gomock.Any(), gomock.Any()).DoAndReturn(img.store.readerAt).AnyTimes()

	return containerd.NewLayerSource(&containerd.Config{Namespace: "unit_test_ns"}, client)
}

func TestLayerSource_Layers(t *testing.T) {
	g.RegisterTestingT(t)

	img := newTestImage(t, []string{ocispec.MediaTypeImageLayerGzip, ocispec.MediaTypeImageLayer}, nil)
	source := layerSourceWithMocks(t, img)
	ctx := context.Background()

	layers, err := source.Layers(ctx, overlay.LayerInput{ImageRef: testImage, Owner: testOwner})
	g.Expect(err).NotTo(g.HaveOccurred())

	// The chain ID of two diff IDs is sha256(diffID1 + " " + diffID2).
	want := digest.FromString(img.diffIDs[0].String() + " " + img.diffIDs[1].String())
	g.Expect(layers.ChainID).To(g.Equal(want))
	g.Expect(layers.Layers).To(g.HaveLen(2))

	for i, layer := range layers.Layers {
		g.Expect(layer.DiffID).To(g.Equal(img.diffIDs[i]))

		for range 2 {
			stream, err := layer.Open(ctx)
			g.Expect(err).NotTo(g.HaveOccurred())

			data, err := io.ReadAll(stream)
			g.Expect(err).NotTo(g.HaveOccurred())
			g.Expect(stream.Close()).To(g.Succeed())
			g.Expect(data).To(g.Equal(img.tars[i]), "layer %d is the uncompressed tar", i)
			g.Expect(digest.FromBytes(data)).To(g.Equal(layer.DiffID))
		}
	}
}

func TestLayerSource_LayerCountMismatch(t *testing.T) {
	g.RegisterTestingT(t)

	only := []digest.Digest{digest.FromString("one")}
	img := newTestImage(t, []string{ocispec.MediaTypeImageLayerGzip, ocispec.MediaTypeImageLayer}, only)
	source := layerSourceWithMocks(t, img)

	_, err := source.Layers(context.Background(), overlay.LayerInput{ImageRef: testImage, Owner: testOwner})
	g.Expect(err).To(g.MatchError(g.ContainSubstring("manifest has 2 layers, config has 1 diff IDs")))
}

func TestLayerSource_NotALayer(t *testing.T) {
	g.RegisterTestingT(t)

	img := newTestImage(t, []string{ocispec.MediaTypeImageLayerGzip, ocispec.MediaTypeImageConfig}, nil)
	source := layerSourceWithMocks(t, img)

	_, err := source.Layers(context.Background(), overlay.LayerInput{ImageRef: testImage, Owner: testOwner})
	g.Expect(err).To(g.MatchError(g.ContainSubstring("layer 1 has media type " + ocispec.MediaTypeImageConfig)))
}

func TestLayerSource_PullFails(t *testing.T) {
	g.RegisterTestingT(t)

	mockCtrl := gomock.NewController(t)
	client := mock.NewMockClient(mockCtrl)
	leasesManager := mock.NewMockManager(mockCtrl)
	boom := errors.New("boom")

	leasesManager.EXPECT().List(gomock.Any(), gomock.Any())
	leasesManager.EXPECT().Create(gomock.Any(), gomock.Any())
	client.EXPECT().LeasesService().Return(leasesManager)
	client.EXPECT().Pull(gomock.Any(), testImage).Return(nil, boom)

	source := containerd.NewLayerSource(&containerd.Config{Namespace: "unit_test_ns"}, client)

	_, err := source.Layers(context.Background(), overlay.LayerInput{ImageRef: testImage, Owner: testOwner})
	g.Expect(errors.Is(err, boom)).To(g.BeTrue(), "got %v", err)
}
