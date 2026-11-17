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

// NewPauseStep returns the step that pauses the VMM of a microvm whose client
// wants it paused. It pauses a VMM that runs again, such as one that the create
// step started after the host restarted.
func NewPauseStep(vm *models.MicroVM, vmSvc ports.MicroVMService) planner.Procedure {
	return &pauseStep{
		vm:    vm,
		vmSvc: vmSvc,
	}
}

type pauseStep struct {
	vm    *models.MicroVM
	vmSvc ports.MicroVMService
}

// Name is the name of the procedure/operation.
func (s *pauseStep) Name() string {
	return "microvm_pause"
}

// ShouldDo is true when the client wants the microvm paused and its VMM runs.
func (s *pauseStep) ShouldDo(ctx context.Context) (bool, error) {
	if !s.vm.Spec.WantsPaused() {
		return false, nil
	}

	state, err := s.vmSvc.State(ctx, s.vm.ID.String())
	if err != nil {
		return false, fmt.Errorf("checking if microvm is running: %w", err)
	}

	return state == ports.MicroVMStateRunning, nil
}

// Do will perform the operation/procedure.
func (s *pauseStep) Do(ctx context.Context) ([]planner.Procedure, error) {
	if s.vm == nil {
		return nil, errors.ErrSpecRequired
	}

	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step": s.Name(),
		"vmid": s.vm.ID,
	})
	logger.Debug("pausing microvm")

	if err := s.vmSvc.Pause(ctx, s.vm); err != nil {
		return nil, fmt.Errorf("pausing microvm: %w", err)
	}

	return nil, nil
}

// Verify checks that the VMM is paused.
func (s *pauseStep) Verify(ctx context.Context) error {
	state, err := s.vmSvc.State(ctx, s.vm.ID.String())
	if err != nil {
		return fmt.Errorf("checking if microvm is paused: %w", err)
	}

	if state != ports.MicroVMStatePaused {
		return fmt.Errorf("microvm %s is not paused after the pause (state: %s)", s.vm.ID, state)
	}

	return nil
}
