package plans

import (
	"context"
	"fmt"
	"path"

	"github.com/weaveworks/flintlock/api/events"
	"github.com/weaveworks/flintlock/core/models"
	"github.com/weaveworks/flintlock/core/ports"
	portsctx "github.com/weaveworks/flintlock/core/ports/context"
	"github.com/weaveworks/flintlock/core/steps/event"
	"github.com/weaveworks/flintlock/core/steps/microvm"
	"github.com/weaveworks/flintlock/core/steps/network"
	"github.com/weaveworks/flintlock/core/steps/runtime"
	"github.com/weaveworks/flintlock/pkg/defaults"
	"github.com/weaveworks/flintlock/pkg/log"
	"github.com/weaveworks/flintlock/pkg/planner"
)

type DeletePlanInput struct {
	StateDirectory string
	VM             *models.MicroVM
}

func MicroVMDeletePlan(input *DeletePlanInput) planner.Plan {
	return &microvmDeletePlan{
		vm:       input.VM,
		stateDir: path.Join(input.StateDirectory, "vm", input.VM.ID.String()),
		steps:    []planner.Procedure{},
	}
}

type microvmDeletePlan struct {
	vm       *models.MicroVM
	stateDir string

	steps []planner.Procedure
}

func (p *microvmDeletePlan) Name() string {
	return MicroVMDeletePlanName
}

// Create will create a plan to delete a microvm.
func (p *microvmDeletePlan) Create(ctx context.Context) ([]planner.Procedure, error) {
	logger := log.GetLogger(ctx)
	logger.Debugf("deleting plan for microvm %s", p.vm.ID)

	ports, ok := portsctx.GetPorts(ctx)
	if !ok {
		return nil, portsctx.ErrPortsMissing
	}

	if p.vm.Spec.DeletedAt == 0 {
		return []planner.Procedure{}, nil
	}

	p.clearPlanList()

	// MicroVM provider delete
	if err := p.addStep(ctx, microvm.NewDeleteStep(p.vm, ports.Provider)); err != nil {
		return nil, fmt.Errorf("adding microvm delete step: %w", err)
	}

	if err := p.addStep(ctx, runtime.NewRepoRelease(p.vm, ports.Repo)); err != nil {
		return nil, fmt.Errorf("adding release lease step: %w", err)
	}

	// Network interfaces
	if err := p.addNetworkSteps(ctx, p.vm, ports.NetworkService); err != nil {
		return nil, fmt.Errorf("adding network steps: %w", err)
	}

	if err := p.addStep(ctx, runtime.NewDeleteDirectory(p.stateDir, ports.FileSystem)); err != nil {
		return nil, fmt.Errorf("adding root dir step: %w", err)
	}

	if len(p.steps) != 0 {
		publishStep := event.NewPublish(
			defaults.TopicMicroVMEvents,
			&events.MicroVMSpecDeleted{
				ID:        p.vm.ID.Name(),
				Namespace: p.vm.ID.Namespace(),
			},
			ports.EventService,
		)
		if err := p.addStep(ctx, publishStep); err != nil {
			return nil, fmt.Errorf("adding publish event: %w", err)
		}
	}

	return p.steps, nil
}

// Result is the result of the plan.
func (p *microvmDeletePlan) Result() interface{} {
	return nil
}

// This is the most important function in the codebase DO NOT REMOVE
// Without this, the Delete will always return the full origin list of steps
// and the State will never be saved, meaning the steps will always return true
// on ShouldDo. The loop will be infinite.
func (p *microvmDeletePlan) clearPlanList() {
	p.steps = []planner.Procedure{}
}

func (p *microvmDeletePlan) addStep(ctx context.Context, step planner.Procedure) error {
	shouldDo, err := step.ShouldDo(ctx)
	if err != nil {
		return fmt.Errorf("checking if step %s should be included in plan: %w", step.Name(), err)
	}

	if shouldDo {
		p.steps = append(p.steps, step)
	}

	return nil
}

func (p *microvmDeletePlan) addNetworkSteps(ctx context.Context, vm *models.MicroVM, networkSvc ports.NetworkService) error {
	for i := range vm.Spec.NetworkInterfaces {
		iface := vm.Spec.NetworkInterfaces[i]

		if err := p.addStep(ctx, network.DeleteNetworkInterface(&vm.ID, &iface, networkSvc)); err != nil {
			return fmt.Errorf("adding delete network interface step: %w", err)
		}
	}

	return nil
}
