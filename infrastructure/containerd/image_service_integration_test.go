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
	testSnapshotter    = "devmapper"
	testOwnerNamespace = "int_ns"
	testOwnerUsageID   = "vol1"
	testOwnerName      = "imageservice-get-test"
	testContainerdNS   = "flintlock_test_ctr"
)

// What this test does?
//
// == Preface
//
// Prepare ImageService and a Namespace context. All images and snapshots will
// live under the same namespace.
//
// We do some prudent cleanup steps in case the test was aborted. Because it's
// an integration test with an external service, if we don't clean up after
// ourselves, the next test run will most likely fail. In theory, releasing the
// leases should be enough, but that's heavily depend on how containerd tells
// us how it works, so it's easier to delete everything we intended to create.
//
// == Chapter I: Pull and mount
//
// Using ImageService, we tries to Pull an image.
//
// We don't have separate Mount function (yet), we try to PullAndMount, with
// this, we can be sure ImageService can pull and image from a repository and
// can mount it.
//
// To avoid false positive result, we try to PullAndMount an image, we know
// it's definitely not there, and we expect it to be failed.  If it does not
// fail, something fishy is swimming under the hood.
//
// After PullAndMount, the real image should be mounted, but the fake one
// shouldn't be. At the end we should have only one image available locally.
//
// == Chapter II: Snapshots
//
// As we already pull and mounted an image, we expect one snapshot to be there
// with the name constructed from the VM namespace, the VM name, and the volume
// usage type.
// At the same time, we expect one lease to be there with similar naming
// structure except the volume usage type we have one lease for all resources
// in containerd.
//
// == Chapter III: Kernel
//
// We do the same checks for kernel. The kernel and volume works the same way,
// and the real reason why we test it here because the usage type is different,
// and we should be able to pull and mount them.
//
// == Chapter IV: Delete
//
// We should be able to delete both the volume and the kernel image. After we
// delete them, we should see zero images in containerd.
func TestImageService_Integration(t *testing.T) {
	if !runContainerDTests() {
		t.Skip("skipping containerd image service integration test")
	}

	RegisterTestingT(t)

	//
	// Preface
	//

	client, ctx := testCreateClient(t)
	namespaceCtx := namespaces.WithNamespace(ctx, testContainerdNS)

	imageSvc := containerd.NewImageServiceWithClient(&containerd.Config{
		SnapshotterKernel: testSnapshotter,
		SnapshotterVolume: testSnapshotter,
		Namespace:         testContainerdNS,
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
	expectedSnapshotName := fmt.Sprintf(
		"flintlock/%s/%s/%s",
		testOwnerNamespace,
		testOwnerName,
		testOwnerUsageID,
	)
	expectedLeaseName := fmt.Sprintf("flintlock/%s/%s", testOwnerNamespace, testOwnerName)

	defer func() {
		// Make sure it's deleted.
		client.ImageService().Delete(namespaceCtx, getTestKernelImage())
		client.ImageService().Delete(namespaceCtx, getTestVolumeImage())
		client.SnapshotService(testSnapshotter).Remove(namespaceCtx, expectedSnapshotName)
		leases, _ := client.LeasesService().List(namespaceCtx)
		for _, lease := range leases {
			client.LeasesService().Delete(namespaceCtx, lease)
		}
	}()

	//
	// Chapter I: Pull and mount.
	//

	err := imageSvc.Pull(ctx, inputGet)
	Expect(err).NotTo(HaveOccurred())

	mounts, err := imageSvc.PullAndMount(ctx, inputGetAndMount)
	Expect(err).NotTo(HaveOccurred())
	Expect(mounts).NotTo(BeNil())
	Expect(len(mounts)).To(Equal(1))

	fakePull := &ports.ImageMountSpec{
		ImageName:    "random/whynot/definitely-not-there",
		Owner:        fmt.Sprintf("%s/%s", testOwnerNamespace, testOwnerName),
		OwnerUsageID: testOwnerUsageID,
		Use:          models.ImageUseVolume,
	}
	mounts, err = imageSvc.PullAndMount(ctx, fakePull)
	Expect(err).To(HaveOccurred())
	Expect(mounts).To(BeNil())

	testImageMounted(ctx, imageSvc, testImageMountOptions{
		ImageName: getTestVolumeImage(),
		Owner:     inputGetAndMount.Owner,
		Use:       models.ImageUseVolume,
		Expected:  true,
	})

	testImageMounted(ctx, imageSvc, testImageMountOptions{
		ImageName: "definitely-not-there",
		Owner:     inputGetAndMount.Owner,
		Use:       models.ImageUseVolume,
		Expected:  false,
	})

	img, err := client.ImageService().List(namespaceCtx)
	Expect(err).NotTo(HaveOccurred())
	Expect(len(img)).To(Equal(1))
	Expect(img[0].Name).To(Equal(getTestVolumeImage()))

	//
	// Chapter II: Snapshots
	//

	snapshotExists := false
	err = client.SnapshotService(testSnapshotter).Walk(namespaceCtx, func(walkCtx context.Context, info snapshots.Info) error {
		if info.Name == expectedSnapshotName {
			snapshotExists = true
		}

		return nil
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(snapshotExists).To(BeTrue(), "expect snapshot with name %s to exist", expectedSnapshotName)

	leases, err := client.LeasesService().List(namespaceCtx)
	Expect(len(leases)).To(Equal(1))
	Expect(leases[0].ID).To(Equal(expectedLeaseName), "expect lease with name %s to exists", expectedLeaseName)

	//
	// Chapter III: Kernel
	//

	inputGet.ImageName = getTestKernelImage()

	err = imageSvc.Pull(ctx, inputGet)
	Expect(err).NotTo(HaveOccurred())

	exists, err := imageSvc.Exists(ctx, &ports.ImageSpec{
		ImageName: getTestVolumeImage(),
		Owner:     testOwnerUsageID,
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(exists).To(BeTrue())

	mounts, err = imageSvc.PullAndMount(ctx, &ports.ImageMountSpec{
		ImageName:    getTestKernelImage(),
		Owner:        testOwnerUsageID,
		Use:          models.ImageUseKernel,
		OwnerUsageID: testOwnerUsageID,
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(mounts).NotTo(BeNil())
	Expect(len(mounts)).To(Equal(1))

	//
	// Chapter IV: Delete
	//

	err = client.ImageService().Delete(namespaceCtx, getTestKernelImage())
	Expect(err).NotTo(HaveOccurred())

	err = client.ImageService().Delete(namespaceCtx, getTestVolumeImage())
	Expect(err).NotTo(HaveOccurred())

	exists, err = imageSvc.Exists(ctx, &ports.ImageSpec{
		ImageName: getTestKernelImage(),
		Owner:     testOwnerUsageID,
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(exists).To(BeFalse())

	images, err := client.ImageService().List(namespaceCtx)
	Expect(err).NotTo(HaveOccurred())
	Expect(len(images)).To(BeZero())
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

// getTestVolumeImage returns with the default test volume image name, if
// CTR_TEST_VOL_IMG environment variable is not defined.
func getTestVolumeImage() string {
	envImage := os.Getenv("CTR_TEST_VOL_IMG")
	if envImage != "" {
		return envImage
	}

	return testImageVolume
}

// getTestKernelImage returns with the default test kernel image name, if
// CTR_TEST_KERNEL_IMG environment variable is not defined.
func getTestKernelImage() string {
	envImage := os.Getenv("CTR_TEST_KERNEL_IMG")
	if envImage != "" {
		return envImage
	}

	return testImageKernel
}

type testImageMountOptions struct {
	ImageName string
	Owner     string
	Use       models.ImageUse
	Expected  bool
}

// testImageMounted checks if we can mount an image.
func testImageMounted(ctx context.Context, imageSvc ports.ImageService, opts testImageMountOptions) {
	mounted, err := imageSvc.IsMounted(ctx, &ports.ImageMountSpec{
		ImageName:    opts.ImageName,
		Owner:        opts.Owner,
		Use:          opts.Use,
		OwnerUsageID: testOwnerUsageID,
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(mounted).To(Equal(opts.Expected))
}
