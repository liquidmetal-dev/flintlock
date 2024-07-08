package microvm

import (
	"context"
	"fmt"

	"github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
	"github.com/liquidmetal-dev/flintlock/pkg/planner"
	"github.com/sirupsen/logrus"
)

func NewCreateStep(vm *models.MicroVM, vmSvc ports.MicroVMService) planner.Procedure {
	return &createStep{
		vm:    vm,
		vmSvc: vmSvc,
	}
}

type createStep struct {
	vm    *models.MicroVM
	vmSvc ports.MicroVMService
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

	return nil, nil
}

func (s *createStep) Verify(ctx context.Context) error {
	return nil
}
