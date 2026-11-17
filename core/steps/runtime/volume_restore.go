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

// NewVolumeRestore returns the step that gives a restored microvm the drives
// of its overlay root volume from the capture in the snapshot directory. It
// records the mounts in the status as NewOverlayVolumeMount does.
func NewVolumeRestore(vmid *models.VMID,
	volume *models.Volume,
	status *models.VolumeStatus,
	snapshotDir string,
	volumeService ports.VolumeService,
	fs afero.Fs,
) planner.Procedure {
	return &volumeRestore{
		overlayVolumeMount: overlayVolumeMount{
			vmid:      vmid,
			volume:    volume,
			status:    status,
			volumeSvc: volumeService,
			fs:        fs,
		},
		snapshotDir: snapshotDir,
	}
}

type volumeRestore struct {
	overlayVolumeMount

	snapshotDir string
}

// Name is the name of the procedure/operation.
func (s *volumeRestore) Name() string {
	return "runtime_volume_restore"
}

// Do will perform the operation/procedure.
func (s *volumeRestore) Do(ctx context.Context) ([]planner.Procedure, error) {
	if s.status == nil {
		return nil, cerrs.ErrMissingStatusInfo
	}

	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step": s.Name(),
		"id":   s.volume.ID,
	})
	logger.Debug("running step to restore overlay volume")

	mounts, err := s.volumeSvc.Restore(ctx, ports.VolumeRestoreInput{
		VolumeRef: ports.VolumeRef{
			VMID:     *s.vmid,
			VolumeID: s.volume.ID,
		},
		SnapshotDir: s.snapshotDir,
		Owner:       s.vmid.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("restoring overlay volume %s from %s: %w", s.volume.ID, s.snapshotDir, err)
	}

	if mounts == nil {
		return nil, cerrs.ErrNoVolumeMount
	}

	s.status.Mount = mounts.RWMount
	s.status.BaseMount = &mounts.BaseMount
	s.status.BaseDigest = mounts.BaseDigest

	return nil, nil
}
