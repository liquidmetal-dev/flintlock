package lifecycle

import (
	"context"
	"fmt"

	reignitev1 "github.com/weaveworks/reignite/api/reignite/v1alpha1"
	"github.com/weaveworks/reignite/pkg/planner"
	"github.com/weaveworks/reignite/pkg/planner/plans/microvm"
)

// Create will create a new microvm.
func (m *microVMLifecycle) Create(ctx context.Context, vm *reignitev1.MicroVM) error {
	createPlan := microvm.NewCreatePlan(&microvm.CreatePlanInput{
		MicroVM:    vm,
		VMProvider: m.microVM,
		State:      m.state,
	})

	actuator := planner.NewActuator()
	if err := actuator.Execute(ctx, createPlan); err != nil {
		return fmt.Errorf("executing create micrvm plan: %w", err)
	}

	return nil
}
