package microvm

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/weaveworks/reignite/core/errors"
	"github.com/weaveworks/reignite/core/models"
	"github.com/weaveworks/reignite/core/ports"
	"github.com/weaveworks/reignite/pkg/log"
	"github.com/weaveworks/reignite/pkg/planner"
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
	isRunning, err := s.vmSvc.IsRunning(ctx, s.vm.ID.String())
	if err != nil {
		return false, fmt.Errorf("checking if microvm is running: %w", err)
	}

	return !isRunning, nil
}

// Do will perform the operation/procedure.
func (s *createStep) Do(ctx context.Context) ([]planner.Procedure, error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step": s.Name(),
		"vmid": s.vm.ID,
	})
	logger.Debug("creating microvm")

	if s.vm == nil {
		return nil, errors.ErrSpecRequired
	}

	if err := s.vmSvc.Create(ctx, s.vm); err != nil {
		return nil, fmt.Errorf("creating microvm: %w", err)
	}

	return nil, nil
}
