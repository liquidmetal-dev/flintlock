package containerd_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/weaveworks/reignite/core/models"
	"github.com/weaveworks/reignite/core/ports"
	"github.com/weaveworks/reignite/infrastructure/containerd"
	"github.com/weaveworks/reignite/pkg/defaults"

	ctr "github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/snapshots"
)

const (
	testImageVolume    = "docker.io/library/alpine:3.14.1"
	testImageKernel    = "docker.io/linuxkit/kernel:5.4.129"
	testSnapshotter    = "native"
	testOwnerNamespace = "int_ns"
	testOwnerName      = "imageservice-get-test"
)

func TestImageService_Integration(t *testing.T) {
	if !runContainerDTests() {
		t.Skip("skipping containerd image service integration test")
	}

	RegisterTestingT(t)

	client, ctx := testCreateClient(t)
	namespaceCtx := namespaces.WithNamespace(ctx, defaults.ContainerdNamespace)

	imageSvc := containerd.NewImageServiceWithClient(&containerd.Config{
		SnapshotterKernel: testSnapshotter,
		SnapshotterVolume: testSnapshotter,
	}, client)

	input := ports.GetImageInput{
		ImageName:      getTestVolumeImage(),
		OwnerName:      testOwnerName,
		OwnerNamespace: testOwnerNamespace,
		Use:            models.ImageUseVolume,
	}
	err := imageSvc.Get(ctx, input)
	Expect(err).NotTo(HaveOccurred())

	mounts, err := imageSvc.GetAndMount(ctx, input)
	Expect(err).NotTo(HaveOccurred())
	Expect(mounts).NotTo(BeNil())
	Expect(len(mounts)).To(Equal(1))

	img, err := client.ImageService().List(namespaceCtx)
	Expect(err).NotTo(HaveOccurred())
	Expect(len(img)).To(Equal(1))
	Expect(img[0].Name).To(Equal(getTestVolumeImage()))

	expectedSnapshotName := fmt.Sprintf("reignite/%s", testOwnerName)
	snapshotExists := false
	err = client.SnapshotService(testSnapshotter).Walk(namespaceCtx, func(walkCtx context.Context, info snapshots.Info) error {
		if info.Name == expectedSnapshotName {
			snapshotExists = true
		}

		return nil
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(snapshotExists).To(BeTrue(), "expect snapshot with name %s to exist", expectedSnapshotName)

	expectedLeaseName := fmt.Sprintf("reignite/%s/%s", testOwnerNamespace, testOwnerName)
	leases, err := client.LeasesService().List(namespaceCtx)
	Expect(err).NotTo(HaveOccurred())
	Expect(len(leases)).To(Equal(1))
	Expect(leases[0].ID).To(Equal(expectedLeaseName), "expect lease with name %s to exists", expectedLeaseName)

	input.Use = models.ImageUseKernel
	input.ImageName = getTestKernelImage()

	err = imageSvc.Get(ctx, input)
	Expect(err).NotTo(HaveOccurred())
}

func testCreateClient(t *testing.T) (*ctr.Client, context.Context) {
	addr := os.Getenv("CTR_SOCK_PATH")
	client, err := ctr.New(addr)
	Expect(err).NotTo(HaveOccurred())

	ctx := context.Background()

	serving, err := client.IsServing(ctx)
	Expect(err).NotTo(HaveOccurred())
	Expect(serving).To(BeTrue())

	return client, ctx
}

func runContainerDTests() bool {
	testCtr := os.Getenv("CTR_SOCK_PATH")
	return testCtr != ""
}

func getTestVolumeImage() string {
	envImage := os.Getenv("CTR_TEST_VOL_IMG")
	if envImage != "" {
		return envImage
	}

	return testImageVolume
}

func getTestKernelImage() string {
	envImage := os.Getenv("CTR_TEST_KERNEL_IMG")
	if envImage != "" {
		return envImage
	}

	return testImageKernel
}
