package microvm

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
	"github.com/liquidmetal-dev/flintlock/pkg/planner"
)

// NewRestoreStep returns the step that starts the VMM of a microvm from the
// snapshot in snapshotDir, in place of the create and start steps.
func NewRestoreStep(
	vm *models.MicroVM,
	vmSvc ports.MicroVMService,
	snapshotDir string,
	wait WaitConfig,
) planner.Procedure {
	return &restoreStep{
		vm:          vm,
		vmSvc:       vmSvc,
		snapshotDir: snapshotDir,
		wait:        wait,
	}
}

type restoreStep struct {
	vm          *models.MicroVM
	vmSvc       ports.MicroVMService
	snapshotDir string
	wait        WaitConfig
}

// Name is the name of the procedure/operation.
func (s *restoreStep) Name() string {
	return "microvm_restore"
}

func (s *restoreStep) ShouldDo(ctx context.Context) (bool, error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step": s.Name(),
		"vmid": s.vm.ID,
	})
	logger.Debug("checking if procedure should be run")

	state, err := s.vmSvc.State(ctx, s.vm.ID.String())
	if err != nil {
		return false, fmt.Errorf("checking if microvm is running: %w", err)
	}

	return state == ports.MicroVMStatePending, nil
}

// Do will perform the operation/procedure.
func (s *restoreStep) Do(ctx context.Context) ([]planner.Procedure, error) {
	if s.vm == nil || s.vm.Spec.SnapshotSource == nil {
		return nil, errors.ErrSpecRequired
	}

	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step": s.Name(),
		"vmid": s.vm.ID,
	})
	logger.Debugf("restoring microvm from %s", s.snapshotDir)

	resetConditions(s.vm, s.wait)

	input := ports.RestoreInput{MemoryMode: s.vm.Spec.SnapshotSource.MemoryMode}
	if err := s.vmSvc.Restore(ctx, s.vm, s.snapshotDir, input); err != nil {
		return nil, fmt.Errorf("restoring microvm from %s: %w", s.snapshotDir, err)
	}

	return nil, nil
}

// Verify waits for the VMM, as the create step does.
func (s *restoreStep) Verify(ctx context.Context) error {
	if s.vm == nil {
		return errors.ErrSpecRequired
	}

	return verifyRunning(ctx, s.vm, s.vmSvc, s.wait)
}
