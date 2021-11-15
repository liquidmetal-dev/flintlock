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

func NewDeleteStep(vm *models.MicroVM, vmSvc ports.MicroVMService) planner.Procedure {
	return &deleteStep{
		vm:    vm,
		vmSvc: vmSvc,
	}
}

type deleteStep struct {
	vm    *models.MicroVM
	vmSvc ports.MicroVMService
}

// Name is the name of the procedure/operation.
func (s *deleteStep) Name() string {
	return "microvm_delete"
}

func (s *deleteStep) ShouldDo(ctx context.Context) (bool, error) {
	state, err := s.vmSvc.State(ctx, s.vm.ID.String())
	if err != nil {
		return false, fmt.Errorf("checking if microvm is running: %w", err)
	}

	stopped := (state == ports.MicroVMStatePending || state == ports.MicroVMStateUnknown)

	return !stopped, nil
}

// Do will perform the operation/procedure.
func (s *deleteStep) Do(ctx context.Context) ([]planner.Procedure, error) {
	if s.vm == nil {
		return nil, errors.ErrSpecRequired
	}

	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step": s.Name(),
		"vmid": s.vm.ID,
	})
	logger.Debug("deleting microvm")

	id := s.vm.ID.String()
	if err := s.vmSvc.Delete(ctx, id); err != nil {
		return nil, fmt.Errorf("deleting microvm: %w", err)
	}

	return nil, nil
}

func (s *deleteStep) Verify(ctx context.Context) error {
	return nil
}
