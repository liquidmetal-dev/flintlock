package containerd

import (
	"fmt"

	"github.com/containerd/containerd/mount"
	"github.com/weaveworks/reignite/core/models"
)

func convertMountToModel(m mount.Mount, snapshotter string) (models.Mount, error) {
	switch snapshotter {
	case "overlayfs":
		return models.Mount{
			Type:   models.MountTypeHostPath,
			Source: getOverlayMountPath(m),
		}, nil
	case "native":
		return models.Mount{
			Type:   models.MountTypeHostPath,
			Source: m.Source,
		}, nil
	case "devmapper":
		return models.Mount{
			Type:   models.MountTypeDev,
			Source: m.Source,
		}, nil
	default:
		return models.Mount{}, errUnsupportedSnapshotter{name: snapshotter}
	}
}

func getOverlayMountPath(m mount.Mount) string {
	return ""
}

func convertMountsToModel(mounts []mount.Mount, snapshotter string) ([]models.Mount, error) {
	convertedMounts := []models.Mount{}
	for _, m := range mounts {
		counvertedMount, err := convertMountToModel(m, snapshotter)
		if err != nil {
			return nil, fmt.Errorf("converting mount: %w", err)
		}
		convertedMounts = append(convertedMounts, counvertedMount)
	}

	return convertedMounts, nil
}
