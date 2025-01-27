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

func NewVirtioFSMount(vmid *models.VMID,
	volume *models.Volume,
	status *models.VolumeStatus,
	vfsSvc ports.VirtioFSService,
) planner.Procedure {
	return &volumeVirtioFSMount{
		vmid:       vmid,
		volume:     volume,
		status:     status,
		vFSService: vfsSvc,
	}
}

type volumeVirtioFSMount struct {
	vmid       *models.VMID
	volume     *models.Volume
	status     *models.VolumeStatus
	vFSService ports.VirtioFSService
}

// Name is the name of the procedure/operation.
func (s *volumeVirtioFSMount) Name() string {
	return "runtime_virtiofs_create"
}

func (s *volumeVirtioFSMount) ShouldDo(ctx context.Context) (bool, error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step": s.Name(),
		"id":   s.volume.ID,
	})
	logger.Debug("checking if procedure should be run")

	if s.status == nil || s.status.Mount.Source == "" {
		return true, nil
	}

	return false, nil
}

// Do will perform the operation/procedure.
func (s *volumeVirtioFSMount) Do(ctx context.Context) ([]planner.Procedure, error) {
	if s.status == nil {
		return nil, cerrs.ErrMissingStatusInfo
	}

	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step": s.Name(),
		"id":   s.volume.ID,
	})
	logger.Trace("Creating VirtioFS")
	vol := ports.VirtioFSCreateInput{
		Path: s.volume.Source.VirtioFS.Path,
	}
	mount, err := s.vFSService.Create(ctx, s.vmid, vol)
	if err != nil {
		return nil, fmt.Errorf("creating microvm: %w", err)
	}
	if mount != nil {
		s.status.Mount = *mount
	}
	return nil, nil
}

func (s *volumeVirtioFSMount) Verify(_ context.Context) error {
	return nil
}
