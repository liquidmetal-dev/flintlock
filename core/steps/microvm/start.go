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

func NewStartStep(vm *models.MicroVM, vmSvc ports.MicroVMService, wait WaitConfig) planner.Procedure {
	return &startStep{
		vm:    vm,
		vmSvc: vmSvc,
		wait:  wait,
	}
}

type startStep struct {
	vm    *models.MicroVM
	vmSvc ports.MicroVMService
	wait  WaitConfig
}

// Name is the name of the procedure/operation.
func (s *startStep) Name() string {
	return "microvm_start"
}

func (s *startStep) ShouldDo(ctx context.Context) (bool, error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step": s.Name(),
		"vmid": s.vm.ID,
	})
	logger.Debug("checking if procedure should be run")

	state, err := s.vmSvc.State(ctx, s.vm.ID.String())
	if err != nil {
		return false, fmt.Errorf("checking if microvm is running: %w", err)
	}

	return !isStarted(state), nil
}

// Do will perform the operation/procedure.
func (s *startStep) Do(ctx context.Context) ([]planner.Procedure, error) {
	if s.vm == nil {
		return nil, errors.ErrSpecRequired
	}

	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step": s.Name(),
		"vmid": s.vm.ID,
	})
	logger.Debug("starting microvm")

	resetConditions(s.vm, s.wait)

	if err := s.vmSvc.Start(ctx, s.vm); err != nil {
		return nil, fmt.Errorf("starting microvm: %w", err)
	}

	return nil, nil
}

func (s *startStep) Verify(ctx context.Context) error {
	if s.vm == nil {
		return errors.ErrSpecRequired
	}

	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step": s.Name(),
		"vmid": s.vm.ID,
	})
	logger.Debug("waiting for the microvm to start")

	return verifyRunning(ctx, s.vm, s.vmSvc, s.wait)
}

// isStarted reports whether a microvm has booted. A paused microvm has booted,
// so it needs no start.
func isStarted(state ports.MicroVMState) bool {
	return state == ports.MicroVMStateRunning || state == ports.MicroVMStatePaused
}
