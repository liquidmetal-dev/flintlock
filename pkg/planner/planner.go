package planner

import "context"

// NOTE: this is based on this prior work https://gianarb.it/blog/reactive-plan-golang-example
// which has been adapted for use here.

// Plan represents an interface for a plan of operations.
type Plan interface {
	// Name is the name of the plan.
	Name() string

	// Create will perform the plan and will return a list of operations/procedures
	// that need to be run to accomplish the plan
	Create(ctx context.Context) ([]Procedure, error)
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
	// Verify the state after Do.
	Verify(ctx context.Context) error
}
