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

	s.resumeIfPaused(ctx, logger)

	if err := s.vmSvc.Delete(ctx, id); err != nil {
		return nil, fmt.Errorf("deleting microvm: %w", err)
	}

	return nil, nil
}

func (s *deleteStep) Verify(_ context.Context) error {
	return nil
}

// resumeIfPaused resumes a paused VMM before the delete, so that the provider
// does not have to handle a paused VMM: Cloud Hypervisor, for one, shuts the
// VM down through its API. A failed resume is logged, and the delete goes on.
func (s *deleteStep) resumeIfPaused(ctx context.Context, logger *logrus.Entry) {
	state, err := s.vmSvc.State(ctx, s.vm.ID.String())
	if err != nil {
		logger.WithError(err).Warn("checking if microvm is paused before the delete")

		return
	}

	if state != ports.MicroVMStatePaused {
		return
	}

	logger.Info("resuming paused microvm before the delete")

	if err := s.vmSvc.Resume(ctx, s.vm); err != nil {
		logger.WithError(err).Warn("resuming paused microvm before the delete")
	}
}
