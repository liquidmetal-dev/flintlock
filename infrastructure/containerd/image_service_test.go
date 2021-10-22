package containerd_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	ctr "github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/snapshots"
	. "github.com/onsi/gomega"

	"github.com/weaveworks/flintlock/core/models"
	"github.com/weaveworks/flintlock/core/ports"
	"github.com/weaveworks/flintlock/infrastructure/containerd"
)

const (
	testImageVolume    = "docker.io/library/alpine:3.14.1"
	testImageKernel    = "docker.io/linuxkit/kernel:5.4.129"
	testSnapshotter    = "native"
	testOwnerNamespace = "int_ns"
	testOwnerUsageID   = "vol1"
	testOwnerName      = "imageservice-get-test"
	testContainerdNs   = "flintlock_test_ctr"
)

func TestImageService_Integration(t *testing.T) {
	if !runContainerDTests() {
		t.Skip("skipping containerd image service integration test")
	}

	RegisterTestingT(t)

	client, ctx := testCreateClient(t)
	namespaceCtx := namespaces.WithNamespace(ctx, testContainerdNs)

	imageSvc := containerd.NewImageServiceWithClient(&containerd.Config{
		SnapshotterKernel: testSnapshotter,
		SnapshotterVolume: testSnapshotter,
		Namespace:         testContainerdNs,
	}, client)

	inputGetAndMount := &ports.ImageMountSpec{
		ImageName:    getTestVolumeImage(),
		Owner:        fmt.Sprintf("%s/%s", testOwnerNamespace, testOwnerName),
		OwnerUsageID: testOwnerUsageID,
		Use:          models.ImageUseVolume,
	}
	inputGet := &ports.ImageSpec{
		ImageName: inputGetAndMount.ImageName,
		Owner:     inputGetAndMount.Owner,
	}

	err := imageSvc.Pull(ctx, inputGet)
	Expect(err).NotTo(HaveOccurred())

	mounts, err := imageSvc.PullAndMount(ctx, inputGetAndMount)
	Expect(err).NotTo(HaveOccurred())
	Expect(mounts).NotTo(BeNil())
	Expect(len(mounts)).To(Equal(1))

	img, err := client.ImageService().List(namespaceCtx)
	Expect(err).NotTo(HaveOccurred())
	Expect(len(img)).To(Equal(1))
	Expect(img[0].Name).To(Equal(getTestVolumeImage()))

	expectedSnapshotName := fmt.Sprintf("flintlock/%s/%s/%s", testOwnerNamespace, testOwnerName, testOwnerUsageID)
	snapshotExists := false
	err = client.SnapshotService(testSnapshotter).Walk(namespaceCtx, func(walkCtx context.Context, info snapshots.Info) error {
		if info.Name == expectedSnapshotName {
			snapshotExists = true
		}

		return nil
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(snapshotExists).To(BeTrue(), "expect snapshot with name %s to exist", expectedSnapshotName)

	expectedLeaseName := fmt.Sprintf("flintlock/%s/%s", testOwnerNamespace, testOwnerName)
	leases, err := client.LeasesService().List(namespaceCtx)
	Expect(err).NotTo(HaveOccurred())
	Expect(len(leases)).To(Equal(1))
	Expect(leases[0].ID).To(Equal(expectedLeaseName), "expect lease with name %s to exists", expectedLeaseName)

	inputGet.ImageName = testImageKernel

	err = imageSvc.Pull(ctx, inputGet)
	Expect(err).NotTo(HaveOccurred())

	err = client.ImageService().Delete(namespaceCtx, testImageKernel)
	Expect(err).NotTo(HaveOccurred())

	err = client.ImageService().Delete(namespaceCtx, testImageVolume)
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
