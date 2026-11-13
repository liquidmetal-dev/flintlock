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

func NewCreateStep(vm *models.MicroVM, vmSvc ports.MicroVMService, wait WaitConfig) planner.Procedure {
	return &createStep{
		vm:    vm,
		vmSvc: vmSvc,
		wait:  wait,
	}
}

type createStep struct {
	vm    *models.MicroVM
	vmSvc ports.MicroVMService
	wait  WaitConfig
}

// Name is the name of the procedure/operation.
func (s *createStep) Name() string {
	return "microvm_create"
}

func (s *createStep) ShouldDo(ctx context.Context) (bool, error) {
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
func (s *createStep) Do(ctx context.Context) ([]planner.Procedure, error) {
	if s.vm == nil {
		return nil, errors.ErrSpecRequired
	}

	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step": s.Name(),
		"vmid": s.vm.ID,
	})
	logger.Debug("creating microvm")

	if err := s.vmSvc.Create(ctx, s.vm); err != nil {
		return nil, fmt.Errorf("creating microvm: %w", err)
	}

	resetGuestAgentReady(s.vm, s.wait)

	return nil, nil
}

// Verify waits for the VMM, because both providers start it in Create.
func (s *createStep) Verify(ctx context.Context) error {
	if s.vm == nil {
		return errors.ErrSpecRequired
	}

	return verifyRunning(ctx, s.vm, s.vmSvc, s.wait)
}
