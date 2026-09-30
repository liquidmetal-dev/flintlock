package containerd_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/errdefs"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	coreerrs "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/infrastructure/containerd"
	"github.com/liquidmetal-dev/flintlock/infrastructure/mock"
)

// The repository reads a spec in two steps: it walks the content store for the
// digest, and then reads the blob. When a MicroVM is deleted between the two,
// containerd has removed the blob, and the read says that it is not found.
// This is what happens when a MicroVM of the namespace is deleted while it is
// listed (#1277).

const deletedSpecNS = "ns-deleted"

// specBlob is a spec in the content store of the fake, as the walk and the
// read see it.
type specBlob struct {
	vm      *models.MicroVM
	readErr error
}

func TestMicroVMRepo_GetAll_LeavesOutASpecDeletedDuringTheList(t *testing.T) {
	g := NewWithT(t)

	kept := makeSpec("kept", deletedSpecNS, "uid-kept")
	deleted := makeSpec("deleted", deletedSpecNS, "uid-deleted")

	repo := repoWithBlobs(t,
		specBlob{vm: kept},
		specBlob{vm: deleted, readErr: fmt.Errorf("content digest x: %w", errdefs.ErrNotFound)},
	)

	all, err := repo.GetAll(context.Background(), models.ListMicroVMQuery{"namespace": deletedSpecNS})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(all).To(HaveLen(1))
	g.Expect(all[0].ID.Name()).To(Equal("kept"))
}

func TestMicroVMRepo_GetAll_ReturnsOtherReadErrors(t *testing.T) {
	g := NewWithT(t)

	broken := makeSpec("broken", deletedSpecNS, "uid-broken")

	repo := repoWithBlobs(t,
		specBlob{vm: broken, readErr: io.ErrUnexpectedEOF},
	)

	_, err := repo.GetAll(context.Background(), models.ListMicroVMQuery{"namespace": deletedSpecNS})
	g.Expect(err).To(HaveOccurred())
	g.Expect(errors.Is(err, io.ErrUnexpectedEOF)).To(BeTrue(), "the cause is kept: %v", err)
	g.Expect(errors.Is(err, containerd.ErrReadingContent)).To(BeTrue())
}

func TestMicroVMRepo_Get_SpecDeletedDuringTheGetIsNotFound(t *testing.T) {
	g := NewWithT(t)

	deleted := makeSpec("deleted", deletedSpecNS, "uid-deleted")

	repo := repoWithBlobs(t,
		specBlob{vm: deleted, readErr: fmt.Errorf("content digest x: %w", errdefs.ErrNotFound)},
	)

	_, err := repo.Get(context.Background(), ports.RepositoryGetOptions{
		Name:      "deleted",
		Namespace: deletedSpecNS,
	})
	g.Expect(err).To(HaveOccurred())
	g.Expect(coreerrs.IsSpecNotFound(err)).To(BeTrue(), "expected a spec not found error, got: %v", err)
}

// repoWithBlobs returns a repository over a fake content store which holds
// the blobs. The walk lists every blob, and the read of a blob returns its
// readErr when it has one.
func repoWithBlobs(t *testing.T, blobs ...specBlob) ports.MicroVMRepository {
	t.Helper()

	ctrl := gomock.NewController(t)

	infos := []content.Info{}
	data := map[digest.Digest][]byte{}
	readErrs := map[digest.Digest]error{}

	for _, blob := range blobs {
		encoded, err := json.Marshal(blob.vm)
		if err != nil {
			t.Fatalf("encoding the spec of %s: %s", blob.vm.ID, err)
		}

		d := digest.FromBytes(encoded)
		data[d] = encoded
		readErrs[d] = blob.readErr
		infos = append(infos, content.Info{
			Digest: d,
			Size:   int64(len(encoded)),
			Labels: map[string]string{
				containerd.NameLabel():      blob.vm.ID.Name(),
				containerd.NamespaceLabel(): blob.vm.ID.Namespace(),
				containerd.TypeLabel():      containerd.MicroVMSpecType,
				containerd.VersionLabel():   fmt.Sprint(blob.vm.Version),
				containerd.UIDLabel():       blob.vm.ID.UID(),
			},
		})
	}

	store := mock.NewMockStore(ctrl)
	store.EXPECT().
		Walk(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, fn content.WalkFunc, _ ...string) error {
			for _, info := range infos {
				if err := fn(info); err != nil {
					return err
				}
			}

			return nil
		}).
		AnyTimes()
	store.EXPECT().
		ReaderAt(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, desc ocispec.Descriptor) (content.ReaderAt, error) {
			if err := readErrs[desc.Digest]; err != nil {
				return nil, err
			}

			encoded, ok := data[desc.Digest]
			if !ok {
				return nil, fmt.Errorf("unknown digest %s: %w", desc.Digest, errdefs.ErrNotFound)
			}

			return blobReader{Reader: bytes.NewReader(encoded)}, nil
		}).
		AnyTimes()

	client := mock.NewMockClient(ctrl)
	client.EXPECT().ContentStore().Return(store).AnyTimes()

	return containerd.NewMicroVMRepoWithClient(&containerd.Config{
		Namespace: "flintlock_test_deleted_spec",
	}, client)
}

// blobReader is a content.ReaderAt over bytes.
type blobReader struct {
	*bytes.Reader
}

func (r blobReader) Close() error {
	return nil
}

func (r blobReader) Size() int64 {
	return r.Reader.Size()
}
