package planner

import (
	"context"

	"github.com/liquidmetal-dev/flintlock/core/models"
)

// NOTE: this is based on this prior work https://gianarb.it/blog/reactive-plan-golang-example
// which has been adapted for use here.

// Plan represents an interface for a plan of operations.
type Plan interface {
	// Name is the name of the plan.
	Name() string

	// Create will perform the plan and will return a list of operations/procedures
	// that need to be run to accomplish the plan
	Create(ctx context.Context) ([]Procedure, error)

	// Finalise will set final status fields when the Plan is complete
	Finalise(state models.MicroVMState)
}

// Procedure represents a procedure/operation that will be carried out
// as part of executing a plan. All procedures must be idempotent, so they
// need to measure and then act.
type Procedure interface {
	// Name is the name of the procedure/operation.
	Name() string
	// Do will perform the operation/procedure.
	Do(ctx context.Context) ([]Procedure, error)
	// ShouldDo determines if this procedure should be executed
	ShouldDo(ctx context.Context) (bool, error)
	// Verify the state after Do. Most cases it can return nil
	// without doing anything, but in special cases we want to measure
	// resources if they are in the desired state.
	// Example: When we start MicroVM, it may does not tell us if it was
	// successful or not, in Verify we can verify if it's running or not
	// and report back an error if the state is not the desired state.
	Verify(ctx context.Context) error
}
