package microvm

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/weaveworks/flintlock/core/errors"
	"github.com/weaveworks/flintlock/core/models"
	"github.com/weaveworks/flintlock/core/ports"
	"github.com/weaveworks/flintlock/pkg/log"
	"github.com/weaveworks/flintlock/pkg/planner"
)

func NewStartStep(vm *models.MicroVM, vmSvc ports.MicroVMService) planner.Procedure {
	return &startStep{
		vm:    vm,
		vmSvc: vmSvc,
	}
}

type startStep struct {
	vm    *models.MicroVM
	vmSvc ports.MicroVMService
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

	return state != ports.MicroVMStateRunning, nil
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

	if err := s.vmSvc.Start(ctx, s.vm.ID.String()); err != nil {
		return nil, fmt.Errorf("starting microvm: %w", err)
	}

	return nil, nil
}
