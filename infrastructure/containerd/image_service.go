package containerd

import (
	"context"
	"fmt"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/namespaces"
	"github.com/sirupsen/logrus"

	"github.com/weaveworks/reignite/core/models"
	"github.com/weaveworks/reignite/core/ports"
	"github.com/weaveworks/reignite/pkg/defaults"
	"github.com/weaveworks/reignite/pkg/log"
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
func NewImageServiceWithClient(cfg *Config, client *containerd.Client) ports.ImageService {
	return &imageService{
		config: cfg,
		client: client,
	}
}

type imageService struct {
	client *containerd.Client
	config *Config
}

// Get will get (i.e. pull) the image for a specific owner.
func (im *imageService) Get(ctx context.Context, input ports.GetImageInput) error {
	logger := log.GetLogger(ctx).WithField("service", "containerd_image")
	actionMessage := fmt.Sprintf("getting image %s for owner %s/%s", input.ImageName, input.OwnerNamespace, input.OwnerName)
	logger.Debugf(actionMessage)

	nsCtx := namespaces.WithNamespace(ctx, defaults.ContainerdNamespace)

	_, err := im.getImage(nsCtx, input.ImageName, input.OwnerName, input.OwnerNamespace)
	if err != nil {
		return fmt.Errorf("%s: %w", actionMessage, err)
	}

	return nil
}

// Get will get (i.e. pull) the image for a specific owner and then
// make it available via a mount point.
func (im *imageService) GetAndMount(ctx context.Context, input ports.GetImageInput) ([]models.Mount, error) {
	logger := log.GetLogger(ctx).WithField("service", "containerd_image")
	logger.Debugf("getting and mounting image %s for owner %s/%s", input.ImageName, input.OwnerNamespace, input.OwnerName)

	nsCtx := namespaces.WithNamespace(ctx, defaults.ContainerdNamespace)

	image, err := im.getImage(nsCtx, input.ImageName, input.OwnerName, input.OwnerNamespace)
	if err != nil {
		return nil, fmt.Errorf("getting image %s for owner %s/%s: %w", input.ImageName, input.OwnerNamespace, input.OwnerName, err)
	}

	ss := im.config.SnapshotterVolume
	if input.Use == models.ImageUseKernel {
		ss = im.config.SnapshotterKernel
	}

	return im.snapshotAndMount(nsCtx, image, input.OwnerName, ss, logger)
}

func (im *imageService) getImage(ctx context.Context, imageName string, ownerName, ownerNamespace string) (containerd.Image, error) {
	leaseCtx, err := withOwnerLease(ctx, ownerName, ownerNamespace, im.client)
	if err != nil {
		return nil, fmt.Errorf("getting lease for owner: %w", err)
	}

	image, err := im.client.Pull(leaseCtx, imageName, containerd.WithPullUnpack)
	if err != nil {
		return nil, fmt.Errorf("pulling image using containerd: %w", err)
	}

	return image, nil
}

func (im *imageService) snapshotAndMount(ctx context.Context, image containerd.Image, ownerName, snapshotter string, logger *logrus.Entry) ([]models.Mount, error) {
	unpacked, err := image.IsUnpacked(ctx, snapshotter)
	if err != nil {
		return nil, fmt.Errorf("checking if image %s has been unpacked with snapshotter %s: %w", image.Name(), snapshotter, err)
	}
	if !unpacked {
		logger.Debugf("image %s isn't unpacked, unpacking using %s snapshotter", image.Name(), snapshotter)
		if unpackErr := image.Unpack(ctx, snapshotter); unpackErr != nil {
			return nil, fmt.Errorf("unpacking %s with snapshotter %s: %w", image.Name(), snapshotter, err)
		}
	}

	imageContent, err := image.RootFS(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting rootfs content for %s: %w", image.Name(), err)
	}
	parent := imageContent[0].String()

	snapshotKey := snapshotKey(ownerName)
	logger.Debugf("creating snapshot %s for image %s with snapshotter %s", snapshotKey, image.Name(), snapshotter)
	ss := im.client.SnapshotService(snapshotter)

	snapshotExists, err := snapshotExists(ctx, snapshotKey, ss)
	if err != nil {
		return nil, fmt.Errorf("checking for existence of snapshot %s: %w", snapshotKey, err)
	}

	var mounts []mount.Mount
	if !snapshotExists {
		mounts, err = ss.Prepare(ctx, snapshotKey, parent)
		if err != nil {
			return nil, fmt.Errorf("preparing snapshot of %s: %w", image.Name(), err)
		}
	} else {
		mounts, err = ss.Mounts(ctx, snapshotKey)
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
