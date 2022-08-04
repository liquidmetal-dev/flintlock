package plans

import (
	"context"
	"fmt"

	"github.com/weaveworks-liquidmetal/flintlock/client/cloudinit"
	"github.com/weaveworks-liquidmetal/flintlock/core/models"
	"github.com/weaveworks-liquidmetal/flintlock/core/ports"
	portsctx "github.com/weaveworks-liquidmetal/flintlock/core/ports/context"
	cisteps "github.com/weaveworks-liquidmetal/flintlock/core/steps/cloudinit"
	"github.com/weaveworks-liquidmetal/flintlock/core/steps/microvm"
	"github.com/weaveworks-liquidmetal/flintlock/core/steps/network"
	"github.com/weaveworks-liquidmetal/flintlock/core/steps/runtime"
	"github.com/weaveworks-liquidmetal/flintlock/pkg/defaults"
	"github.com/weaveworks-liquidmetal/flintlock/pkg/log"
	"github.com/weaveworks-liquidmetal/flintlock/pkg/planner"
)

type CreateOrUpdatePlanInput struct {
	StateDirectory     string
	CloudinitViaVolume bool
	VM                 *models.MicroVM
}

func MicroVMCreateOrUpdatePlan(input *CreateOrUpdatePlanInput) planner.Plan {
	return &microvmCreateOrUpdatePlan{
		vm:                 input.VM,
		stateDir:           input.StateDirectory,
		cloudInitViaVolume: input.CloudinitViaVolume,
		steps:              []planner.Procedure{},
	}
}

type microvmCreateOrUpdatePlan struct {
	vm                 *models.MicroVM
	stateDir           string
	cloudInitViaVolume bool

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

	if p.vm.Spec.DeletedAt != 0 {
		return []planner.Procedure{}, nil
	}

	p.clearPlanList()
	p.ensureStatus()

	if err := p.addStep(ctx, runtime.NewCreateDirectory(p.stateDir, defaults.DataDirPerm, ports.FileSystem)); err != nil {
		return nil, fmt.Errorf("adding root dir step: %w", err)
	}

	// Microvm runtime state dir
	if err := p.addStep(ctx, microvm.NewStateDirStep(p.stateDir, p.vm, ports.FileSystem)); err != nil {
		return nil, fmt.Errorf("adding microvm runtime state directory step: %w", err)
	}

	// Add additional disk for metadata / cloudinit to vm spec
	if err := p.addAdditionVolumeSteps(ctx, p.vm); err != nil {
		return nil, fmt.Errorf("adding additional volumes: %w", err)
	}

	// Add cloud-init vendor to mount additional volumes
	if err := p.addStep(ctx, cisteps.NewDiskMountStep(p.vm)); err != nil {
		return nil, fmt.Errorf("adding disk mount step: %w", err)
	}

	//TODO: create disks for metadata / cloudinit if needed
	if err := p.addCreateDiskSteps(ctx, p.vm, ports); err != nil {
		return nil, fmt.Errorf("adding additional volumes: %w", err)
	}

	// Images
	if err := p.addImageSteps(ctx, p.vm, ports.ImageService); err != nil {
		return nil, fmt.Errorf("adding image steps: %w", err)
	}

	// Network interfaces
	if err := p.addNetworkSteps(ctx, p.vm, ports.NetworkService); err != nil {
		return nil, fmt.Errorf("adding network steps: %w", err)
	}

	// MicroVM provider create
	if err := p.addStep(ctx, microvm.NewCreateStep(p.vm, ports.Provider)); err != nil {
		return nil, fmt.Errorf("adding microvm create step: %w", err)
	}

	// MicroVM provider start
	if err := p.addStep(ctx, microvm.NewStartStep(p.vm, ports.Provider, microVMBootTime)); err != nil {
		return nil, fmt.Errorf("adding microvm start step: %w", err)
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

func (p *microvmCreateOrUpdatePlan) addAdditionVolumeSteps(ctx context.Context,
	vm *models.MicroVM,
) error {

	if p.vm.Spec.Metadata.AddVolume {
		imagePath := fmt.Sprintf("${RUNTIME_STATEDIR}/%s.img", dataVolumeName)
		if err := p.addStep(ctx, microvm.NewAttachVolumeStep(
			vm,
			imagePath,
			dataVolumeName,
			true,
			true,
		)); err != nil {
			return fmt.Errorf("adding attach volume step for metadata disk: %w", err)
		}
	}
	if p.cloudInitViaVolume {
		imagePath := fmt.Sprintf("${RUNTIME_STATEDIR}/%s.img", cloudinit.VolumeName)
		if err := p.addStep(ctx, microvm.NewAttachVolumeStep(
			vm,
			imagePath,
			cloudinit.VolumeName,
			true,
			false,
		)); err != nil {
			return fmt.Errorf("adding attach volume step for cloud-init disk: %w", err)
		}
	}

	return nil
}

func (p *microvmCreateOrUpdatePlan) addCreateDiskSteps(ctx context.Context,
	vm *models.MicroVM,
	portsCol *ports.Collection,
) error {

	if p.vm.Spec.Metadata.AddVolume {
		if err := p.addStep(ctx, microvm.NewCreateVolumeDiskStep(&microvm.CreateVolumeDiskStepInput{
			VM:          p.vm,
			DiskSvc:     portsCol.DiskService,
			FS:          portsCol.FileSystem,
			VolumeID:    dataVolumeName,
			VolumeSize:  "8Mb",
			ContentFunc: diskContentFunc(p.vm, false),
		})); err != nil {
			return fmt.Errorf("adding disk create steps for metadata volume: %w", err)
		}
	}
	if p.cloudInitViaVolume {
		if err := p.addStep(ctx, microvm.NewCreateVolumeDiskStep(&microvm.CreateVolumeDiskStepInput{
			VM:          p.vm,
			DiskSvc:     portsCol.DiskService,
			FS:          portsCol.FileSystem,
			VolumeID:    cloudinit.VolumeName,
			VolumeSize:  "8Mb",
			ContentFunc: diskContentFunc(p.vm, true),
		})); err != nil {
			return fmt.Errorf("adding disk create steps for cloud-init volume: %w", err)
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

func diskContentFunc(vm *models.MicroVM, cloudInitVolume bool) microvm.GetContentFunc {
	return func() map[string]string {
		content := map[string]string{}

		for k, v := range vm.Spec.Metadata.Items {
			isCloudInit := isCloudInitMetadata(k)
			if cloudInitVolume == isCloudInit {
				content[k] = v
			}
		}

		return content
	}
}

func isCloudInitMetadata(keyName string) bool {
	switch keyName {
	case cloudinit.InstanceDataKey:
		return true
	case cloudinit.NetworkConfigDataKey:
		return true
	case cloudinit.UserdataKey:
		return true
	case cloudinit.VendorDataKey:
		return true
	default:
		return false
	}
}
