package plans

import (
	"context"
	"fmt"

	"github.com/liquidmetal-dev/flintlock/core/steps/cloudinit"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	portsctx "github.com/liquidmetal-dev/flintlock/core/ports/context"
	"github.com/liquidmetal-dev/flintlock/core/steps/microvm"
	"github.com/liquidmetal-dev/flintlock/core/steps/network"
	"github.com/liquidmetal-dev/flintlock/core/steps/runtime"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
	"github.com/liquidmetal-dev/flintlock/pkg/planner"
)

type CreateOrUpdatePlanInput struct {
	StateDirectory string
	VM             *models.MicroVM
}

func MicroVMCreateOrUpdatePlan(input *CreateOrUpdatePlanInput) planner.Plan {
	return &microvmCreateOrUpdatePlan{
		vm:       input.VM,
		stateDir: input.StateDirectory,
		steps:    []planner.Procedure{},
	}
}

type microvmCreateOrUpdatePlan struct {
	vm       *models.MicroVM
	stateDir string

	steps []planner.Procedure
}

func (p *microvmCreateOrUpdatePlan) Name() string {
	return MicroVMCreateOrUpdatePlanName
}

func (p *microvmCreateOrUpdatePlan) Create(ctx context.Context) ([]planner.Procedure, error) {
	logger := log.GetLogger(ctx).WithField("component", "plans").WithField("planType", "microvmCreateOrUpdatePlan")
	logger.Tracef("creating CreateOrUpdate plan for microvm: %s", p.vm.ID)

	ports, ok := portsctx.GetPorts(ctx)
	if !ok {
		return nil, portsctx.ErrPortsMissing
	}

	provider, ok := ports.MicrovmProviders[p.vm.Spec.Provider]
	if !ok {
		return nil, fmt.Errorf("microvm provider %s isn't available", p.vm.Spec.Provider)
	}

	if p.vm.Spec.DeletedAt != 0 {
		return []planner.Procedure{}, nil
	}

	p.clearPlanList()
	p.ensureStatus()

	if err := p.addStep(ctx, runtime.NewCreateDirectory(p.stateDir, defaults.DataDirPerm, ports.FileSystem)); err != nil {
		return nil, fmt.Errorf("adding root dir step: %w", err)
	}

	// Images
	if err := p.addImageSteps(ctx, p.vm, ports.ImageService); err != nil {
		return nil, fmt.Errorf("adding image steps: %w", err)
	}
	if len(p.vm.Spec.AdditionalVolumes) > 0 {
		if err := p.addStep(ctx, cloudinit.NewDiskMountStep(p.vm)); err != nil {
			return nil, fmt.Errorf("adding mount step: %w", err)
		}
	}

	// Network interfaces
	if err := p.addNetworkSteps(ctx, p.vm, ports.NetworkService); err != nil {
		return nil, fmt.Errorf("adding network steps: %w", err)
	}

	// MicroVM provider create
	if err := p.addStep(ctx, microvm.NewCreateStep(p.vm, provider)); err != nil {
		return nil, fmt.Errorf("adding microvm create step: %w", err)
	}

	// MicroVM provider doesn't auto-start
	if !provider.Capabilities().Has(models.AutoStartCapability) {
		if err := p.addStep(ctx, microvm.NewStartStep(p.vm, provider, microVMBootTime)); err != nil {
			return nil, fmt.Errorf("adding microvm start step: %w", err)
		}
	}

	return p.steps, nil
}

func (p *microvmCreateOrUpdatePlan) Finalise(state models.MicroVMState) {
	p.vm.Status.State = state
}

// This is the most important function in the codebase DO NOT REMOVE
// Without this, the Create will always return the full origin list of steps
// and the State will never be saved, meaning the steps will always return true
// on ShouldDo. The loop will be infinite.
func (p *microvmCreateOrUpdatePlan) clearPlanList() {
	p.steps = []planner.Procedure{}
}

func (p *microvmCreateOrUpdatePlan) addStep(ctx context.Context, step planner.Procedure) error {
	shouldDo, err := step.ShouldDo(ctx)
	if err != nil {
		return fmt.Errorf("checking if step %s should be included in plan: %w", step.Name(), err)
	}

	if shouldDo {
		p.steps = append(p.steps, step)
	}

	return nil
}

func (p *microvmCreateOrUpdatePlan) addImageSteps(ctx context.Context,
	vm *models.MicroVM,
	imageSvc ports.ImageService,
) error {
	rootStatus, ok := vm.Status.Volumes[vm.Spec.RootVolume.ID]
	if !ok {
		rootStatus = &models.VolumeStatus{}
		vm.Status.Volumes[vm.Spec.RootVolume.ID] = rootStatus
	}

	if vm.Spec.RootVolume.Source.Container != nil {
		if err := p.addStep(ctx, runtime.NewVolumeMount(&vm.ID, &vm.Spec.RootVolume, rootStatus, imageSvc)); err != nil {
			return fmt.Errorf("adding root volume mount step: %w", err)
		}
	}

	for i := range vm.Spec.AdditionalVolumes {
		vol := vm.Spec.AdditionalVolumes[i]

		status, ok := vm.Status.Volumes[vol.ID]
		if !ok {
			status = &models.VolumeStatus{}
			vm.Status.Volumes[vol.ID] = status
		}

		if vol.Source.Container != nil {
			if err := p.addStep(ctx, runtime.NewVolumeMount(&vm.ID, &vol, status, imageSvc)); err != nil {
				return fmt.Errorf("adding volume mount step: %w", err)
			}
		}
	}

	if string(vm.Spec.Kernel.Image) != "" {
		if err := p.addStep(ctx, runtime.NewKernelMount(vm, imageSvc)); err != nil {
			return fmt.Errorf("adding kernel mount step: %w", err)
		}
	}

	if vm.Spec.Initrd != nil {
		if err := p.addStep(ctx, runtime.NewInitrdMount(vm, imageSvc)); err != nil {
			return fmt.Errorf("adding initrd mount step: %w", err)
		}
	}

	return nil
}

func (p *microvmCreateOrUpdatePlan) addNetworkSteps(ctx context.Context,
	vm *models.MicroVM,
	networkSvc ports.NetworkService,
) error {
	for i := range vm.Spec.NetworkInterfaces {
		iface := vm.Spec.NetworkInterfaces[i]

		status, ok := vm.Status.NetworkInterfaces[iface.GuestDeviceName]
		if !ok {
			status = &models.NetworkInterfaceStatus{}
			vm.Status.NetworkInterfaces[iface.GuestDeviceName] = status
		}

		if err := p.addStep(ctx, network.NewNetworkInterface(&vm.ID, &iface, status, networkSvc)); err != nil {
			return fmt.Errorf("adding create network interface step: %w", err)
		}
	}

	return nil
}

func (p *microvmCreateOrUpdatePlan) ensureStatus() {
	if p.vm.Status.Volumes == nil {
		p.vm.Status.Volumes = models.VolumeStatuses{}
	}

	if p.vm.Status.NetworkInterfaces == nil {
		p.vm.Status.NetworkInterfaces = models.NetworkInterfaceStatuses{}
	}

	// If we are going through the create/update steps, then switch to pending first.
	// When all is done and successful it will be put to created.
	p.vm.Status.State = models.PendingState
}
