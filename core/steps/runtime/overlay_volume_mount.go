package runtime

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"

	cerrs "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
	"github.com/liquidmetal-dev/flintlock/pkg/planner"
)

// NewOverlayVolumeMount returns the step that provisions the drives of a root
// volume in VolumeModeOverlay. It records the writable disk as the mount of
// the volume, the base as its base mount, and the digest of the base.
func NewOverlayVolumeMount(vmid *models.VMID,
	volume *models.Volume,
	status *models.VolumeStatus,
	volumeService ports.VolumeService,
	fs afero.Fs,
) planner.Procedure {
	return &overlayVolumeMount{
		vmid:      vmid,
		volume:    volume,
		status:    status,
		volumeSvc: volumeService,
		fs:        fs,
	}
}

type overlayVolumeMount struct {
	vmid      *models.VMID
	volume    *models.Volume
	status    *models.VolumeStatus
	volumeSvc ports.VolumeService
	fs        afero.Fs
}

// Name is the name of the procedure/operation.
func (s *overlayVolumeMount) Name() string {
	return "runtime_overlay_volume_mount"
}

// ShouldDo reports whether the drives need to be provisioned. They do not
// when the status records both mounts and both files exist on the host.
func (s *overlayVolumeMount) ShouldDo(ctx context.Context) (bool, error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step": s.Name(),
		"id":   s.volume.ID,
	})
	logger.Debug("checking if procedure should be run")

	if s.status == nil || s.status.Mount.Source == "" {
		return true, nil
	}

	if s.status.BaseMount == nil || s.status.BaseMount.Source == "" {
		return true, nil
	}

	for _, path := range []string{s.status.Mount.Source, s.status.BaseMount.Source} {
		exists, err := afero.Exists(s.fs, path)
		if err != nil {
			return false, fmt.Errorf("checking if %s exists: %w", path, err)
		}

		if !exists {
			logger.Debugf("%s is missing, volume needs provisioning", path)

			return true, nil
		}
	}

	return false, nil
}

// Do will perform the operation/procedure.
func (s *overlayVolumeMount) Do(ctx context.Context) ([]planner.Procedure, error) {
	if s.status == nil {
		return nil, cerrs.ErrMissingStatusInfo
	}

	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step": s.Name(),
		"id":   s.volume.ID,
	})
	logger.Debug("running step to provision overlay volume")

	input := s.getCreateInput()

	mounts, err := s.volumeSvc.Create(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("provisioning overlay volume from %s: %w", input.ImageRef, err)
	}

	if mounts == nil {
		return nil, cerrs.ErrNoVolumeMount
	}

	s.status.Mount = mounts.RWMount
	s.status.BaseMount = &mounts.BaseMount
	s.status.BaseDigest = mounts.BaseDigest

	return nil, nil
}

func (s *overlayVolumeMount) getCreateInput() ports.VolumeCreateInput {
	return ports.VolumeCreateInput{
		VolumeRef: ports.VolumeRef{
			VMID:     *s.vmid,
			VolumeID: s.volume.ID,
		},
		ImageRef: string(s.volume.Source.Container.Image),
		SizeMB:   s.volume.Size,
		Owner:    s.vmid.String(),
	}
}

func (s *overlayVolumeMount) Verify(_ context.Context) error {
	return nil
}
