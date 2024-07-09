package containerd_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/infrastructure/containerd"
	"github.com/liquidmetal-dev/flintlock/infrastructure/mock"
	g "github.com/onsi/gomega"
	"github.com/opencontainers/go-digest"
)

const (
	testImage   = "testimage"
	testOwner   = "testowner"
	testOwnerID = "testownerid"
)

// TestImageService_Pull tests a successful Pull.
func TestImageService_Pull(t *testing.T) {
	g.RegisterTestingT(t)

	mockCtrl := gomock.NewController(t)
	containerdClient := mock.NewMockClient(mockCtrl)
	leasesManager := mock.NewMockManager(mockCtrl)
	svcConfig := containerd.Config{
		SnapshotterKernel: "native",
		SnapshotterVolume: "devmapper",
		SocketPath:        "/something",
		Namespace:         "unit_test_ns",
	}
	ctx := context.Background()
	client := containerd.NewImageServiceWithClient(&svcConfig, containerdClient)

	leasesManager.EXPECT().
		List(gomock.Any(), fmt.Sprintf("id==flintlock/%s", testOwner))
	leasesManager.EXPECT().
		Create(gomock.Any(), gomock.Any())
	containerdClient.EXPECT().
		LeasesService().
		Return(leasesManager)
	containerdClient.EXPECT().
		Pull(gomock.Any(), testImage)

	err := client.Pull(ctx, &ports.ImageSpec{
		ImageName: testImage,
		Owner:     testOwner,
	})
	g.Expect(err).NotTo(g.HaveOccurred())
}

// TestImageService_Pull_failedLease tests what happens when something goes
// wrong with Leases.
func TestImageService_Pull_failedLease(t *testing.T) {
	g.RegisterTestingT(t)

	mockCtrl := gomock.NewController(t)
	containerdClient := mock.NewMockClient(mockCtrl)
	leasesManager := mock.NewMockManager(mockCtrl)
	svcConfig := containerd.Config{
		SnapshotterKernel: "native",
		SnapshotterVolume: "devmapper",
		SocketPath:        "/something",
		Namespace:         "unit_test_ns",
	}
	ctx := context.Background()
	client := containerd.NewImageServiceWithClient(&svcConfig, containerdClient)

	leasesManager.EXPECT().
		List(gomock.Any(), fmt.Sprintf("id==flintlock/%s", testOwner)).
		Return(nil, errors.New("nope"))
	containerdClient.EXPECT().
		LeasesService().
		Return(leasesManager)

	err := client.Pull(ctx, &ports.ImageSpec{
		ImageName: testImage,
		Owner:     testOwner,
	})
	g.Expect(err).To(g.HaveOccurred())
}

// TestImageService_PullAndMount tests a successful PullAndMount.
func TestImageService_PullAndMount(t *testing.T) {
	g.RegisterTestingT(t)

	mockCtrl := gomock.NewController(t)
	containerdClient := mock.NewMockClient(mockCtrl)
	leasesManager := mock.NewMockManager(mockCtrl)
	snapshotManager := mock.NewMockSnapshotter(mockCtrl)
	image := mock.NewMockImage(mockCtrl)
	svcConfig := containerd.Config{
		SnapshotterKernel: "native",
		SnapshotterVolume: "devmapper",
		SocketPath:        "/something",
		Namespace:         "unit_test_ns",
	}
	ctx := context.Background()
	client := containerd.NewImageServiceWithClient(&svcConfig, containerdClient)
	hash := "randomhash"

	leasesManager.EXPECT().
		List(gomock.Any(), fmt.Sprintf("id==flintlock/%s", testOwner)).
		Times(2)
	leasesManager.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Times(2)
	snapshotManager.EXPECT().
		Walk(gomock.Any(), gomock.Any())
	snapshotManager.EXPECT().
		Prepare(
			gomock.Any(),
			fmt.Sprintf("flintlock/%s/%s", testOwner, testOwnerID),
			hash,
			gomock.Any(),
		)
	containerdClient.EXPECT().
		LeasesService().
		Return(leasesManager).
		Times(2)
	containerdClient.EXPECT().
		SnapshotService("devmapper").
		Return(snapshotManager).
		Times(1)
	image.EXPECT().
		IsUnpacked(gomock.Any(), "devmapper").
		Return(false, nil)
	image.EXPECT().
		Name().
		Return(testImage).
		Times(2)
	image.EXPECT().
		Unpack(gomock.Any(), "devmapper").
		Return(nil)
	image.EXPECT().
		RootFS(gomock.Any()).
		Return([]digest.Digest{digest.Digest(hash)}, nil)
	containerdClient.EXPECT().
		Pull(gomock.Any(), testImage).
		Return(image, nil)

	_, err := client.PullAndMount(ctx, &ports.ImageMountSpec{
		ImageName:    testImage,
		Owner:        testOwner,
		Use:          models.ImageUseVolume,
		OwnerUsageID: testOwnerID,
	})
	g.Expect(err).NotTo(g.HaveOccurred())
}

// TestImageService_PullAndMount_failedLease tests what happens when something
// goes wrong with Leases.
func TestImageService_PullAndMount_failedLease(t *testing.T) {
	g.RegisterTestingT(t)

	mockCtrl := gomock.NewController(t)
	containerdClient := mock.NewMockClient(mockCtrl)
	leasesManager := mock.NewMockManager(mockCtrl)
	svcConfig := containerd.Config{
		SnapshotterKernel: "native",
		SnapshotterVolume: "devmapper",
		SocketPath:        "/something",
		Namespace:         "unit_test_ns",
	}
	ctx := context.Background()
	client := containerd.NewImageServiceWithClient(&svcConfig, containerdClient)

	leasesManager.EXPECT().
		List(gomock.Any(), fmt.Sprintf("id==flintlock/%s", testOwner)).
		Return(nil, errors.New("nope"))
	containerdClient.EXPECT().
		LeasesService().
		Return(leasesManager)

	_, err := client.PullAndMount(ctx, &ports.ImageMountSpec{
		ImageName:    testImage,
		Owner:        testOwner,
		Use:          models.ImageUseVolume,
		OwnerUsageID: testOwnerID,
	})
	g.Expect(err).To(g.HaveOccurred())
}

// TestImageService_PullAndMount_failedLeaseOnImageCheck tests what happens
// when something goes wrong with Leases on imageExists.
func TestImageService_PullAndMount_failedLeaseOnImageCheck(t *testing.T) {
	g.RegisterTestingT(t)

	mockCtrl := gomock.NewController(t)
	containerdClient := mock.NewMockClient(mockCtrl)
	leasesManager := mock.NewMockManager(mockCtrl)
	svcConfig := containerd.Config{
		SnapshotterKernel: "native",
		SnapshotterVolume: "devmapper",
		SocketPath:        "/something",
		Namespace:         "unit_test_ns",
	}
	ctx := context.Background()
	client := containerd.NewImageServiceWithClient(&svcConfig, containerdClient)

	leasesManager.EXPECT().
		List(gomock.Any(), fmt.Sprintf("id==flintlock/%s", testOwner))
	leasesManager.EXPECT().
		List(gomock.Any(), fmt.Sprintf("id==flintlock/%s", testOwner)).
		Return(nil, errors.New("nope"))
	leasesManager.EXPECT().
		Create(gomock.Any(), gomock.Any())
	containerdClient.EXPECT().
		LeasesService().
		Return(leasesManager).
		Times(2)

	_, err := client.PullAndMount(ctx, &ports.ImageMountSpec{
		ImageName:    testImage,
		Owner:        testOwner,
		Use:          models.ImageUseVolume,
		OwnerUsageID: testOwnerID,
	})
	g.Expect(err).To(g.HaveOccurred())
}

// TestImageService_PullAndMount_failedUnpackCheck tests what happens when
// something goes wrong with unpack check.
func TestImageService_PullAndMount_failedUnpackCheck(t *testing.T) {
	g.RegisterTestingT(t)

	mockCtrl := gomock.NewController(t)
	containerdClient := mock.NewMockClient(mockCtrl)
	leasesManager := mock.NewMockManager(mockCtrl)
	image := mock.NewMockImage(mockCtrl)
	svcConfig := containerd.Config{
		SnapshotterKernel: "native",
		SnapshotterVolume: "devmapper",
		SocketPath:        "/something",
		Namespace:         "unit_test_ns",
	}
	ctx := context.Background()
	client := containerd.NewImageServiceWithClient(&svcConfig, containerdClient)

	leasesManager.EXPECT().
		List(gomock.Any(), fmt.Sprintf("id==flintlock/%s", testOwner)).
		Times(2)
	leasesManager.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Times(2)
	image.EXPECT().
		Name().
		Return(testImage)
	image.EXPECT().
		IsUnpacked(gomock.Any(), "devmapper").
		Return(false, errors.New("nope"))
	containerdClient.EXPECT().
		LeasesService().
		Return(leasesManager).
		Times(2)
	containerdClient.EXPECT().
		Pull(gomock.Any(), testImage).
		Return(image, nil)

	_, err := client.PullAndMount(ctx, &ports.ImageMountSpec{
		ImageName:    testImage,
		Owner:        testOwner,
		Use:          models.ImageUseVolume,
		OwnerUsageID: testOwnerID,
	})
	g.Expect(err).To(g.HaveOccurred())
}

// TestImageService_PullAndMount_failedUnpackCheck tests what happens when
// something goes wrong with unpack.
func TestImageService_PullAndMount_failedUnpack(t *testing.T) {
	g.RegisterTestingT(t)

	mockCtrl := gomock.NewController(t)
	containerdClient := mock.NewMockClient(mockCtrl)
	leasesManager := mock.NewMockManager(mockCtrl)
	image := mock.NewMockImage(mockCtrl)
	svcConfig := containerd.Config{
		SnapshotterKernel: "native",
		SnapshotterVolume: "devmapper",
		SocketPath:        "/something",
		Namespace:         "unit_test_ns",
	}
	ctx := context.Background()
	client := containerd.NewImageServiceWithClient(&svcConfig, containerdClient)

	leasesManager.EXPECT().
		List(gomock.Any(), fmt.Sprintf("id==flintlock/%s", testOwner)).
		Times(2)
	leasesManager.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Times(2)
	image.EXPECT().
		Name().
		Return(testImage).
		Times(2)
	image.EXPECT().
		IsUnpacked(gomock.Any(), "devmapper").
		Return(false, nil)
	image.EXPECT().
		Unpack(gomock.Any(), "devmapper").
		Return(errors.New("nope"))
	containerdClient.EXPECT().
		LeasesService().
		Return(leasesManager).
		Times(2)
	containerdClient.EXPECT().
		Pull(gomock.Any(), testImage).
		Return(image, nil)

	_, err := client.PullAndMount(ctx, &ports.ImageMountSpec{
		ImageName:    testImage,
		Owner:        testOwner,
		Use:          models.ImageUseVolume,
		OwnerUsageID: testOwnerID,
	})
	g.Expect(err).To(g.HaveOccurred())
}

// TestImageService_PullAndMount_failedRootFS tests what happens when
// something goes wrong with retrieving RootFS.
func TestImageService_PullAndMount_failedRootFS(t *testing.T) {
	g.RegisterTestingT(t)

	mockCtrl := gomock.NewController(t)
	containerdClient := mock.NewMockClient(mockCtrl)
	leasesManager := mock.NewMockManager(mockCtrl)
	image := mock.NewMockImage(mockCtrl)
	svcConfig := containerd.Config{
		SnapshotterKernel: "native",
		SnapshotterVolume: "devmapper",
		SocketPath:        "/something",
		Namespace:         "unit_test_ns",
	}
	ctx := context.Background()
	client := containerd.NewImageServiceWithClient(&svcConfig, containerdClient)

	leasesManager.EXPECT().
		List(gomock.Any(), fmt.Sprintf("id==flintlock/%s", testOwner)).
		Times(2)
	leasesManager.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Times(2)
	containerdClient.EXPECT().
		Pull(gomock.Any(), testImage).
		Return(image, nil)
	image.EXPECT().
		Name().
		Return(testImage).
		Times(2)
	image.EXPECT().
		IsUnpacked(gomock.Any(), "devmapper").
		Return(false, nil)
	image.EXPECT().
		Unpack(gomock.Any(), "devmapper").
		Return(nil)
	image.EXPECT().
		RootFS(gomock.Any()).
		Return([]digest.Digest{}, errors.New("nope"))
	containerdClient.EXPECT().
		LeasesService().
		Return(leasesManager).
		Times(2)

	_, err := client.PullAndMount(ctx, &ports.ImageMountSpec{
		ImageName:    testImage,
		Owner:        testOwner,
		Use:          models.ImageUseVolume,
		OwnerUsageID: testOwnerID,
	})
	g.Expect(err).To(g.HaveOccurred())
}

// TestImageService_PullAndMount_failedSnapshotCheck tests what happens when
// something goes wrong checking the snapshot.
func TestImageService_PullAndMount_failedSnapshotCheck(t *testing.T) {
	g.RegisterTestingT(t)

	mockCtrl := gomock.NewController(t)
	containerdClient := mock.NewMockClient(mockCtrl)
	leasesManager := mock.NewMockManager(mockCtrl)
	snapshotManager := mock.NewMockSnapshotter(mockCtrl)
	image := mock.NewMockImage(mockCtrl)
	svcConfig := containerd.Config{
		SnapshotterKernel: "native",
		SnapshotterVolume: "devmapper",
		SocketPath:        "/something",
		Namespace:         "unit_test_ns",
	}
	ctx := context.Background()
	client := containerd.NewImageServiceWithClient(&svcConfig, containerdClient)
	hash := "randomhash"

	leasesManager.EXPECT().
		List(gomock.Any(), fmt.Sprintf("id==flintlock/%s", testOwner)).
		Times(2)
	leasesManager.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Times(2)
	snapshotManager.EXPECT().
		Walk(gomock.Any(), gomock.Any()).
		Return(errors.New("nope"))
	image.EXPECT().
		Name().
		Return(testImage).
		Times(2)
	image.EXPECT().
		IsUnpacked(gomock.Any(), "devmapper").
		Return(false, nil)
	image.EXPECT().
		Unpack(gomock.Any(), "devmapper").
		Return(nil)
	image.EXPECT().
		RootFS(gomock.Any()).
		Return([]digest.Digest{digest.Digest(hash)}, nil)
	containerdClient.EXPECT().
		SnapshotService("devmapper").
		Return(snapshotManager).
		Times(1)
	containerdClient.EXPECT().
		LeasesService().
		Return(leasesManager).
		Times(2)
	containerdClient.EXPECT().
		Pull(gomock.Any(), testImage).
		Return(image, nil)

	_, err := client.PullAndMount(ctx, &ports.ImageMountSpec{
		ImageName:    testImage,
		Owner:        testOwner,
		Use:          models.ImageUseVolume,
		OwnerUsageID: testOwnerID,
	})
	g.Expect(err).To(g.HaveOccurred())
}

// TestImageService_PullAndMount_failedPrepare tests what happens when
// something goes wrong preparing the snapshot.
func TestImageService_PullAndMount_failedPrepare(t *testing.T) {
	g.RegisterTestingT(t)

	mockCtrl := gomock.NewController(t)
	containerdClient := mock.NewMockClient(mockCtrl)
	leasesManager := mock.NewMockManager(mockCtrl)
	snapshotManager := mock.NewMockSnapshotter(mockCtrl)
	image := mock.NewMockImage(mockCtrl)
	svcConfig := containerd.Config{
		SnapshotterKernel: "native",
		SnapshotterVolume: "devmapper",
		SocketPath:        "/something",
		Namespace:         "unit_test_ns",
	}
	ctx := context.Background()
	client := containerd.NewImageServiceWithClient(&svcConfig, containerdClient)
	hash := "randomhash"

	leasesManager.EXPECT().
		List(gomock.Any(), fmt.Sprintf("id==flintlock/%s", testOwner)).
		Times(2)
	leasesManager.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Times(2)
	snapshotManager.EXPECT().
		Walk(gomock.Any(), gomock.Any())
	snapshotManager.EXPECT().
		Prepare(
			gomock.Any(),
			fmt.Sprintf("flintlock/%s/%s", testOwner, testOwnerID),
			hash,
			gomock.Any(),
		).
		Return(nil, errors.New("nope"))
	image.EXPECT().
		Name().
		Return(testImage).
		AnyTimes()
	image.EXPECT().
		IsUnpacked(gomock.Any(), "devmapper").
		Return(false, nil)
	image.EXPECT().
		Unpack(gomock.Any(), "devmapper").
		Return(nil)
	image.EXPECT().
		RootFS(gomock.Any()).
		Return([]digest.Digest{digest.Digest(hash)}, nil)
	containerdClient.EXPECT().
		SnapshotService("devmapper").
		Return(snapshotManager).
		Times(1)
	containerdClient.EXPECT().
		LeasesService().
		Return(leasesManager).
		Times(2)
	containerdClient.EXPECT().
		Pull(gomock.Any(), testImage).
		Return(image, nil)

	_, err := client.PullAndMount(ctx, &ports.ImageMountSpec{
		ImageName:    testImage,
		Owner:        testOwner,
		Use:          models.ImageUseVolume,
		OwnerUsageID: testOwnerID,
	})
	g.Expect(err).To(g.HaveOccurred())
}

// TestImageService_IsMounted_failedImageCheck tests what happens when
// something goes wrong with imageExists.
func TestImageService_IsMounted_failedImageCheck(t *testing.T) {
	g.RegisterTestingT(t)

	mockCtrl := gomock.NewController(t)
	containerdClient := mock.NewMockClient(mockCtrl)
	leasesManager := mock.NewMockManager(mockCtrl)
	svcConfig := containerd.Config{
		SnapshotterKernel: "native",
		SnapshotterVolume: "devmapper",
		SocketPath:        "/something",
		Namespace:         "unit_test_ns",
	}
	ctx := context.Background()
	client := containerd.NewImageServiceWithClient(&svcConfig, containerdClient)

	leasesManager.EXPECT().
		List(gomock.Any(), fmt.Sprintf("id==flintlock/%s", testOwner))
	leasesManager.EXPECT().
		Create(gomock.Any(), gomock.Any())
	containerdClient.EXPECT().
		LeasesService().
		Return(leasesManager)
	containerdClient.EXPECT().
		GetImage(gomock.Any(), testImage).
		Return(nil, errors.New("nope"))

	_, err := client.IsMounted(ctx, &ports.ImageMountSpec{
		ImageName:    testImage,
		Owner:        testOwner,
		Use:          models.ImageUseVolume,
		OwnerUsageID: testOwnerID,
	})
	g.Expect(err).To(g.HaveOccurred())
}

// TestImageService_IsMounted_failedSnapshotCheck tests what happens when
// something goes wrong with snapshotExists.
func TestImageService_IsMounted_failedSnapshotCheck(t *testing.T) {
	g.RegisterTestingT(t)

	mockCtrl := gomock.NewController(t)
	containerdClient := mock.NewMockClient(mockCtrl)
	leasesManager := mock.NewMockManager(mockCtrl)
	snapshotManager := mock.NewMockSnapshotter(mockCtrl)
	image := mock.NewMockImage(mockCtrl)
	svcConfig := containerd.Config{
		SnapshotterKernel: "native",
		SnapshotterVolume: "devmapper",
		SocketPath:        "/something",
		Namespace:         "unit_test_ns",
	}
	ctx := context.Background()
	client := containerd.NewImageServiceWithClient(&svcConfig, containerdClient)

	leasesManager.EXPECT().
		List(gomock.Any(), fmt.Sprintf("id==flintlock/%s", testOwner)).
		Times(1)
	leasesManager.EXPECT().
		Create(gomock.Any(), gomock.Any()).
		Times(1)
	snapshotManager.EXPECT().
		Walk(gomock.Any(), gomock.Any()).
		Return(errors.New("nope"))
	containerdClient.EXPECT().
		SnapshotService("devmapper").
		Return(snapshotManager).
		Times(1)
	containerdClient.EXPECT().
		LeasesService().
		Return(leasesManager).
		Times(1)
	containerdClient.EXPECT().
		GetImage(gomock.Any(), testImage).
		Return(image, nil)

	_, err := client.IsMounted(ctx, &ports.ImageMountSpec{
		ImageName:    testImage,
		Owner:        testOwner,
		Use:          models.ImageUseVolume,
		OwnerUsageID: testOwnerID,
	})
	g.Expect(err).To(g.HaveOccurred())
}

func TestImageService_Exists_failedCheck(t *testing.T) {
	g.RegisterTestingT(t)

	mockCtrl := gomock.NewController(t)
	containerdClient := mock.NewMockClient(mockCtrl)
	leasesManager := mock.NewMockManager(mockCtrl)
	svcConfig := containerd.Config{
		SnapshotterKernel: "native",
		SnapshotterVolume: "devmapper",
		SocketPath:        "/something",
		Namespace:         "unit_test_ns",
	}
	ctx := context.Background()
	client := containerd.NewImageServiceWithClient(&svcConfig, containerdClient)

	leasesManager.EXPECT().
		List(gomock.Any(), fmt.Sprintf("id==flintlock/%s", testOwner))
	leasesManager.EXPECT().
		Create(gomock.Any(), gomock.Any())
	containerdClient.EXPECT().
		LeasesService().
		Return(leasesManager)
	containerdClient.EXPECT().
		GetImage(gomock.Any(), testImage).
		Return(nil, errors.New("nope"))

	exists, err := client.Exists(ctx, &ports.ImageSpec{
		ImageName: testImage,
		Owner:     testOwner,
	})
	g.Expect(err).To(g.HaveOccurred())
	g.Expect(exists).To(g.BeFalse())
}
