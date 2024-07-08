package runtime

import (
	"context"
	"fmt"

	cerrs "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
	"github.com/liquidmetal-dev/flintlock/pkg/planner"
	"github.com/sirupsen/logrus"
)

func NewVolumeMount(vmid *models.VMID,
	volume *models.Volume,
	status *models.VolumeStatus,
	imageService ports.ImageService,
) planner.Procedure {
	return &volumeMount{
		vmid:     vmid,
		volume:   volume,
		status:   status,
		imageSvc: imageService,
	}
}

type volumeMount struct {
	vmid     *models.VMID
	volume   *models.Volume
	status   *models.VolumeStatus
	imageSvc ports.ImageService
}

// Name is the name of the procedure/operation.
func (s *volumeMount) Name() string {
	return "runtime_volume_mount"
}

func (s *volumeMount) ShouldDo(ctx context.Context) (bool, error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step": s.Name(),
		"id":   s.volume.ID,
	})
	logger.Debug("checking if procedure should be run")

	if s.status == nil || s.status.Mount.Source == "" {
		return true, nil
	}

	input := s.getMountSpec()

	mounted, err := s.imageSvc.IsMounted(ctx, input)
	if err != nil {
		return false, fmt.Errorf("checking if image %s is mounted: %w", input.ImageName, err)
	}

	return !mounted, nil
}

// Do will perform the operation/procedure.
func (s *volumeMount) Do(ctx context.Context) ([]planner.Procedure, error) {
	if s.status == nil {
		return nil, cerrs.ErrMissingStatusInfo
	}

	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step": s.Name(),
		"id":   s.volume.ID,
	})
	logger.Debug("running step to mount volume")

	input := s.getMountSpec()

	mounts, err := s.imageSvc.PullAndMount(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("mount images %s for volume use: %w", input.ImageName, err)
	}

	if len(mounts) == 0 {
		return nil, cerrs.ErrNoVolumeMount
	}

	s.status.Mount = mounts[0]

	return nil, nil
}

func (s *volumeMount) getMountSpec() *ports.ImageMountSpec {
	return &ports.ImageMountSpec{
		ImageName:    string(s.volume.Source.Container.Image),
		Owner:        s.vmid.String(),
		OwnerUsageID: s.volume.ID,
		Use:          models.ImageUseVolume,
	}
}

func (s *volumeMount) Verify(ctx context.Context) error {
	return nil
}
