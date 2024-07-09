package containerd

import (
	"fmt"

	"github.com/containerd/containerd/events"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/typeurl/v2"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
)

func convertMountToModel(mountPoint mount.Mount, snapshotter string) (models.Mount, error) {
	switch snapshotter {
	case "overlayfs":
		return models.Mount{
			Type:   models.MountTypeHostPath,
			Source: getOverlayMountPath(mountPoint),
		}, nil
	case "native":
		return models.Mount{
			Type:   models.MountTypeHostPath,
			Source: mountPoint.Source,
		}, nil
	case "devmapper":
		return models.Mount{
			Type:   models.MountTypeDev,
			Source: mountPoint.Source,
		}, nil
	default:
		return models.Mount{}, unsupportedSnapshotterError{name: snapshotter}
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

func convertCtrEventEnvelope(evt *events.Envelope) (*ports.EventEnvelope, error) {
	if evt == nil {
		return nil, nil
	}

	converted := &ports.EventEnvelope{
		Timestamp: evt.Timestamp,
		Namespace: evt.Namespace,
		Topic:     evt.Topic,
	}

	v, err := typeurl.UnmarshalAny(evt.Event)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling event: %w", err)
	}

	converted.Event = v

	return converted, nil
}
