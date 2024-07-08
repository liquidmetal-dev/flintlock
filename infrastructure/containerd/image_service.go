package containerd

import (
	"context"
	"fmt"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/snapshots"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
	"github.com/opencontainers/image-spec/identity"
	"github.com/sirupsen/logrus"
)

// NewImageService will create a new image service based on containerd with the supplied config.
func NewImageService(cfg *Config) (ports.ImageService, error) {
	client, err := containerd.New(cfg.SocketPath)
	if err != nil {
		return nil, fmt.Errorf("creating containerd client: %w", err)
	}

	return NewImageServiceWithClient(cfg, client), nil
}

// NewImageServiceWithClient will create a new image service based on containerd with the supplied containerd client.
func NewImageServiceWithClient(cfg *Config, client Client) ports.ImageService {
	return &imageService{
		config: cfg,
		client: client,
	}
}

type imageService struct {
	client Client
	config *Config
}

// Pull will get (i.e. pull) the image for a specific owner.
func (im *imageService) Pull(ctx context.Context, input *ports.ImageSpec) error {
	logger := log.GetLogger(ctx).WithField("service", "containerd_image")
	actionMessage := fmt.Sprintf("getting image %s for owner %s", input.ImageName, input.Owner)
	logger.Debugf(actionMessage)

	nsCtx := namespaces.WithNamespace(ctx, im.config.Namespace)

	_, err := im.pullImage(nsCtx, input.ImageName, input.Owner)
	if err != nil {
		return fmt.Errorf("%s: %w", actionMessage, err)
	}

	return nil
}

// PullAndMount will get (i.e. pull) the image for a specific owner and then make it available via a mount point.
func (im *imageService) PullAndMount(ctx context.Context, input *ports.ImageMountSpec) ([]models.Mount, error) {
	logger := log.GetLogger(ctx).WithField("service", "containerd_image")
	logger.Debugf("getting and mounting image %s for owner %s", input.ImageName, input.Owner)

	nsCtx := namespaces.WithNamespace(ctx, im.config.Namespace)

	leaseCtx, err := withOwnerLease(nsCtx, input.Owner, im.client)
	if err != nil {
		return nil, fmt.Errorf("getting lease for image pulling and mounting: %w", err)
	}

	image, err := im.pullImage(leaseCtx, input.ImageName, input.Owner)
	if err != nil {
		return nil, fmt.Errorf("getting image %s for owner %s: %w", input.ImageName, input.Owner, err)
	}

	ss := im.getSnapshotter(input.Use)

	return im.snapshotAndMount(leaseCtx, image, input.Owner, input.OwnerUsageID, ss, logger)
}

// Exists checks if the image already exists on the machine.
func (im *imageService) Exists(ctx context.Context, input *ports.ImageSpec) (bool, error) {
	logger := log.GetLogger(ctx).WithField("service", "containerd_image")
	logger.Debugf("checking if image %s exists for owner %s", input.ImageName, input.Owner)

	nsCtx := namespaces.WithNamespace(ctx, im.config.Namespace)

	exists, err := im.imageExists(nsCtx, input.ImageName, input.Owner)
	if err != nil {
		return false, fmt.Errorf("checking image exists: %w", err)
	}

	return exists, nil
}

// IsMounted checks if the image is pulled and mounted.
func (im *imageService) IsMounted(ctx context.Context, input *ports.ImageMountSpec) (bool, error) {
	logger := log.GetLogger(ctx).WithField("service", "containerd_image")
	logger.Debugf("checking if image %s exists and is mounted for owner %s", input.ImageName, input.Owner)

	nsCtx := namespaces.WithNamespace(ctx, im.config.Namespace)

	exists, err := im.imageExists(nsCtx, input.ImageName, input.Owner)
	if err != nil {
		return false, fmt.Errorf("checking image exists: %w", err)
	}

	if !exists {
		return false, nil
	}

	snapshotter := im.getSnapshotter(input.Use)
	snapshotKey := snapshotKey(input.Owner, input.OwnerUsageID)
	ss := im.client.SnapshotService(snapshotter)

	snapshotExists, err := snapshotExists(nsCtx, snapshotKey, ss)
	if err != nil {
		return false, fmt.Errorf("checking for existence of snapshot %s: %w", snapshotKey, err)
	}

	return snapshotExists, nil
}

func (im *imageService) imageExists(ctx context.Context, imageName, owner string) (bool, error) {
	leaseCtx, err := withOwnerLease(ctx, owner, im.client)
	if err != nil {
		return false, fmt.Errorf("getting lease for owner: %w", err)
	}

	if _, err := im.client.GetImage(leaseCtx, imageName); err != nil {
		if errdefs.IsNotFound(err) {
			return false, nil
		}

		return false, fmt.Errorf("getting image from containerd %s: %w", imageName, err)
	}

	return true, nil
}

func (im *imageService) pullImage(ctx context.Context, imageName string, owner string) (containerd.Image, error) {
	leaseCtx, err := withOwnerLease(ctx, owner, im.client)
	if err != nil {
		return nil, fmt.Errorf("getting lease for owner: %w", err)
	}

	image, err := im.client.Pull(leaseCtx, imageName)
	if err != nil {
		return nil, fmt.Errorf("pulling image using containerd: %w", err)
	}

	return image, nil
}

func (im *imageService) snapshotAndMount(ctx context.Context,
	image containerd.Image,
	owner, ownerUsageID, snapshotter string,
	logger *logrus.Entry,
) ([]models.Mount, error) {
	unpacked, err := image.IsUnpacked(ctx, snapshotter)
	if err != nil {
		return nil, fmt.Errorf("checking if image %s has been unpacked with snapshotter %s: %w",
			image.Name(),
			snapshotter,
			err,
		)
	}

	if !unpacked {
		logger.Debugf("image %s isn't unpacked, unpacking using %s snapshotter", image.Name(), snapshotter)

		if unpackErr := image.Unpack(ctx, snapshotter); unpackErr != nil {
			return nil, fmt.Errorf("unpacking %s with snapshotter %s: %w", image.Name(), snapshotter, unpackErr)
		}
	}

	imageContent, err := image.RootFS(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting rootfs content for %s: %w", image.Name(), err)
	}

	parent := identity.ChainID(imageContent).String()

	snapshotKey := snapshotKey(owner, ownerUsageID)
	logger.Debugf("creating snapshot %s for image %s with snapshotter %s", snapshotKey, image.Name(), snapshotter)
	snapService := im.client.SnapshotService(snapshotter)

	snapshotExists, err := snapshotExists(ctx, snapshotKey, snapService)
	if err != nil {
		return nil, fmt.Errorf("checking for existence of snapshot %s: %w", snapshotKey, err)
	}

	var mounts []mount.Mount

	if !snapshotExists {
		labels := map[string]string{
			"flintlock/owner":       owner,
			"flintlock/owner-usage": ownerUsageID,
		}

		mounts, err = snapService.Prepare(ctx, snapshotKey, parent, snapshots.WithLabels(labels))
		if err != nil {
			return nil, fmt.Errorf("preparing snapshot of %s: %w", image.Name(), err)
		}
	} else {
		mounts, err = snapService.Mounts(ctx, snapshotKey)
		if err != nil {
			return nil, fmt.Errorf("getting mounts of %s: %w", image.Name(), err)
		}
	}

	convertedMounts, err := convertMountsToModel(mounts, snapshotter)
	if err != nil {
		return nil, fmt.Errorf("converting snapshot mounts: %w", err)
	}

	return convertedMounts, nil
}

func (im *imageService) getSnapshotter(use models.ImageUse) string {
	if use == models.ImageUseVolume {
		return im.config.SnapshotterVolume
	}

	return im.config.SnapshotterKernel
}
