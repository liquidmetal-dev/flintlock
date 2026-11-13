package microvm

import (
	"context"
	"fmt"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/pkg/planner"
)

func NewVMMRunningStep(vm *models.MicroVM, vmSvc ports.MicroVMService, wait WaitConfig) planner.Procedure {
	return &vmmRunningStep{
		vm:    vm,
		vmSvc: vmSvc,
		wait:  wait,
	}
}

type vmmRunningStep struct {
	vm    *models.MicroVM
	vmSvc ports.MicroVMService
	wait  WaitConfig
}

// Name is the name of the procedure/operation.
func (s *vmmRunningStep) Name() string {
	return "microvm_vmm_running"
}

// ShouldDo is true for a VMM that runs but has no VMMRunning condition, such
// as a VMM that started before the daemon recorded conditions. The create and
// start steps skip such a VMM, so they do not set the condition.
func (s *vmmRunningStep) ShouldDo(ctx context.Context) (bool, error) {
	if _, ok := s.vm.Status.Conditions.Get(models.ConditionVMMRunning); ok {
		return false, nil
	}

	state, err := s.vmSvc.State(ctx, s.vm.ID.String())
	if err != nil {
		return false, fmt.Errorf("checking if microvm is running: %w", err)
	}

	return state == ports.MicroVMStateRunning, nil
}

func (s *vmmRunningStep) Do(_ context.Context) ([]planner.Procedure, error) {
	setVMMRunning(s.vm, models.ConditionTrue, models.ReasonRunning, "", s.wait)

	return nil, nil
}

func (s *vmmRunningStep) Verify(_ context.Context) error {
	return nil
}
