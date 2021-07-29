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

	// Result is the result of the plan
	Result() interface{}
}

// Procedure represents a procedure/operation that will be carried out
// as part of executing a plan.
type Procedure interface {
	// Name is the name of the procedure/operation.
	Name() string
	// Do will perform the operation/procedure.
	Do(ctx context.Context) ([]Procedure, error)
}
