package plans

import (
	"context"

	"github.com/weaveworks/reignite/core/models"
	"github.com/weaveworks/reignite/pkg/log"
	"github.com/weaveworks/reignite/pkg/planner"
)

type UpdatePlanInput struct {
	StateDirectory string
	VM             *models.MicroVM
}

func MicroVMUpdatePlan(input *UpdatePlanInput) planner.Plan {
	return &microvmUpdatePlan{
		vm:       input.VM,
		stateDir: input.StateDirectory,
		steps:    []planner.Procedure{},
	}
}

type microvmUpdatePlan struct {
	vm       *models.MicroVM
	stateDir string

	steps []planner.Procedure
}

func (p *microvmUpdatePlan) Name() string {
	return MicroVMUpdatePlanName
}

// Create will update the plan to reconcile a microvm.
func (p *microvmUpdatePlan) Create(ctx context.Context) ([]planner.Procedure, error) {
	logger := log.GetLogger(ctx)
	logger.Debugf("updating plan for microvm %s", p.vm.ID)

	return []planner.Procedure{}, nil
}

// Result is the result of the plan.
func (p *microvmUpdatePlan) Result() interface{} {
	return nil
}
