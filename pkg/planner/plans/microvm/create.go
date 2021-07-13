package microvm

import (
	"context"

	reignitev1 "github.com/weaveworks/reignite/api/reignite/v1alpha1"
	"github.com/weaveworks/reignite/pkg/log"
	vmprovider "github.com/weaveworks/reignite/pkg/microvm"
	"github.com/weaveworks/reignite/pkg/planner"
	"github.com/weaveworks/reignite/pkg/planner/steps/microvm"
	statesteps "github.com/weaveworks/reignite/pkg/planner/steps/state"
	"github.com/weaveworks/reignite/pkg/state"
)

type CreatePlanInput struct {
	MicroVM *reignitev1.MicroVM

	VMProvider vmprovider.Provider
	State      state.StateProvider
}

func NewCreatePlan(input *CreatePlanInput) planner.Plan {
	return &createPlan{
		microvm:    input.MicroVM,
		vmprovider: input.VMProvider,
		state:      input.State,
	}
}

type createPlan struct {
	microvm    *reignitev1.MicroVM
	vmprovider vmprovider.Provider
	state      state.StateProvider
}

func (p *createPlan) Name() string {
	return "create_microvm"
}

// Create will perform the plan and will return a list of operations/procedures
// that need to be run to accomplish the plan
func (p *createPlan) Create(ctx context.Context) ([]planner.Procedure, error) {
	logger := log.GetLogger(ctx)

	procs := []planner.Procedure{}

	logger.Trace("checking metadata")
	if needsMetadata(p.microvm) {
		procs = append(procs, microvm.NewPopulateMetadataStep(p.microvm, p.state, logger))
	}
	state := p.state.Get(p.microvm.Name)
	if !state.Exists() {
		procs = append(procs, statesteps.VMStateStep(p.microvm, p.state, logger))
	}

	//TODO: create the networkns

	//TODO: create the cgroup

	//TODO: pull rootfs image
	//TODO: pull kernel image
	//TODO: pull intrd image (optional)

	//TODO: CNI to create network interfaces

	//TODO: create instance of microvm

	return procs, nil
}

// Result is the result of the plan
func (p *createPlan) Result() interface{} {
	return p.microvm
}

func needsMetadata(microvm *reignitev1.MicroVM) bool {
	return microvm.CreationTimestamp.IsZero() || microvm.Name == ""
}
