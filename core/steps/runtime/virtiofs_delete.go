package runtime

import (
	"context"
	"fmt"
	cerrs "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
	"github.com/liquidmetal-dev/flintlock/pkg/planner"
	"github.com/sirupsen/logrus"
	"github.com/liquidmetal-dev/flintlock/core/ports"
)

func NewDeleteVirtioFSMount(vmid *models.VMID,
	volume *models.Volume,
	status *models.VolumeStatus,
	vmSvc ports.MicroVMService,
	vfsSvc ports.VirtioFSService,
) planner.Procedure {
	return &deleteVolumeVirtioFSMount{
		vmid:   vmid,
		volume: volume,
		status: status,
		vFSService: vfsSvc,
		vmSvc: vmSvc,
		vfsSvc: vfsSvc,
	}
}

type deleteVolumeVirtioFSMount struct {
	vmid     *models.VMID
	volume   *models.Volume
	status   *models.VolumeStatus
	vFSService ports.VirtioFSService
	vmSvc ports.MicroVMService
	vfsSvc ports.VirtioFSService
}

// Name is the name of the procedure/operation.
func (s *deleteVolumeVirtioFSMount) Name() string {
	return "runtime_virtiofs_delete"
}

func (s *deleteVolumeVirtioFSMount) ShouldDo(ctx context.Context) (bool, error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step": s.Name(),
		"id":   s.volume.ID,
	})
	logger.Debug("checking if procedure should be run")
	

	return s.vFSService.HasVirtioFSDProcess(ctx,s.vmid)
}

// Do will perform the operation/procedure.
func (s *deleteVolumeVirtioFSMount) Do(ctx context.Context) ([]planner.Procedure, error) {
	if s.status == nil {
		return nil, cerrs.ErrMissingStatusInfo
	}
	if err := s.vFSService.Delete(ctx, s.vmid); err != nil {
		return nil, fmt.Errorf("deleting viritofsd: %w", err)
	}

	return nil,nil
}

func (s *deleteVolumeVirtioFSMount) Verify(_ context.Context) error {
	return nil
}