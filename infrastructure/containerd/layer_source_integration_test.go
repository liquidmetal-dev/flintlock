package containerd_test

import (
	"io"
	"os"
	"testing"

	"github.com/containerd/containerd/v2/core/leases"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	. "github.com/onsi/gomega"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/identity"

	"github.com/liquidmetal-dev/flintlock/infrastructure/containerd"
	"github.com/liquidmetal-dev/flintlock/infrastructure/volume/overlay"
)

// TestLayerSource_Integration pulls the test volume image through a
// containerd daemon and checks that every stream the source returns hashes
// to its diff ID and that the chain ID is the one of the image's rootfs.
func TestLayerSource_Integration(t *testing.T) {
	if !runContainerDTests() {
		t.Skip("skipping containerd layer source integration test")
	}

	RegisterTestingT(t)

	client, ctx := testCreateClient(t)
	namespaceCtx := namespaces.WithNamespace(ctx, testContainerdNS)
	owner := "layersource-test"

	t.Cleanup(func() {
		_ = client.ImageService().Delete(namespaceCtx, getTestVolumeImage())
		_ = client.LeasesService().Delete(namespaceCtx, leases.Lease{ID: "flintlock/" + owner}, leases.SynchronousDelete)
	})

	source := containerd.NewLayerSource(&containerd.Config{
		SocketPath: os.Getenv("CTR_SOCK_PATH"),
		Namespace:  testContainerdNS,
	}, client)

	layers, err := source.Layers(ctx, overlay.LayerInput{ImageRef: getTestVolumeImage(), Owner: owner})
	Expect(err).NotTo(HaveOccurred())
	Expect(layers.Layers).NotTo(BeEmpty())

	image, err := client.GetImage(namespaceCtx, getTestVolumeImage())
	Expect(err).NotTo(HaveOccurred())

	diffIDs, err := image.RootFS(namespaceCtx)
	Expect(err).NotTo(HaveOccurred())
	Expect(layers.ChainID).To(Equal(identity.ChainID(diffIDs)))
	Expect(layers.Layers).To(HaveLen(len(diffIDs)))

	for i, layer := range layers.Layers {
		Expect(layer.DiffID).To(Equal(diffIDs[i]))

		stream, err := layer.Open(ctx)
		Expect(err).NotTo(HaveOccurred())

		digester := digest.Canonical.Digester()
		_, err = io.Copy(digester.Hash(), stream)
		Expect(err).NotTo(HaveOccurred())
		Expect(stream.Close()).To(Succeed())
		Expect(digester.Digest()).To(Equal(layer.DiffID), "layer %d stream hashes to its diff ID", i)
	}

	// The image is held by the owner's lease.
	found, err := client.LeasesService().List(namespaceCtx, "id==flintlock/"+owner)
	Expect(err).NotTo(HaveOccurred())
	Expect(found).To(HaveLen(1))
}
