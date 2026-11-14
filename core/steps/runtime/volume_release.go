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

// NewVolumeRelease returns the step that releases the drives of a root volume
// in VolumeModeOverlay: the writable disk and the base link of the microvm.
// The release also sweeps bases that no microvm uses. The step runs while the
// state directory of the microvm, vmDir, exists, so it needs no volume status:
// a microvm whose create failed early is released too.
func NewVolumeRelease(vm *models.MicroVM,
	vmDir string,
	volumeService ports.VolumeService,
	fs afero.Fs,
) planner.Procedure {
	return &volumeRelease{
		vm:        vm,
		vmDir:     vmDir,
		volumeSvc: volumeService,
		fs:        fs,
	}
}

type volumeRelease struct {
	vm        *models.MicroVM
	vmDir     string
	volumeSvc ports.VolumeService
	fs        afero.Fs
}

// Name is the name of the procedure/operation.
func (s *volumeRelease) Name() string {
	return "runtime_volume_release"
}

// ShouldDo reports whether the root volume is an overlay whose state
// directory still exists.
func (s *volumeRelease) ShouldDo(ctx context.Context) (bool, error) {
	if s.vm == nil {
		return false, cerrs.ErrSpecRequired
	}

	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step": s.Name(),
		"id":   s.vm.Spec.RootVolume.ID,
	})
	logger.Debug("checking if procedure should be run")

	if !s.vm.Spec.RootVolume.IsOverlay() {
		return false, nil
	}

	exists, err := afero.DirExists(s.fs, s.vmDir)
	if err != nil {
		return false, fmt.Errorf("checking if dir %s exists: %w", s.vmDir, err)
	}

	return exists, nil
}

// Do will perform the operation/procedure.
func (s *volumeRelease) Do(ctx context.Context) ([]planner.Procedure, error) {
	if s.vm == nil {
		return nil, cerrs.ErrSpecRequired
	}

	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step": s.Name(),
		"id":   s.vm.Spec.RootVolume.ID,
	})
	logger.Debug("running step to release overlay volume")

	ref := ports.VolumeRef{
		VMID:     s.vm.ID,
		VolumeID: s.vm.Spec.RootVolume.ID,
	}

	if err := s.volumeSvc.Release(ctx, ref); err != nil {
		return nil, fmt.Errorf("releasing overlay volume %s of %s: %w", ref.VolumeID, ref.VMID, err)
	}

	return nil, nil
}

func (s *volumeRelease) Verify(_ context.Context) error {
	return nil
}
