package microvm

import (
	"context"

	reignitev1 "github.com/weaveworks/reignite/api/reignite/v1alpha1"
)

const (
	// ProviderFirecracker is the name of the firecracker microvm provider.
	ProviderFirecracker = "firecracker"
)

// Providers is a list of supported microvm providers.
var Providers = []string{ProviderFirecracker}

// Provider is the interface that a microvm provider needs to implement.
type Provider interface {
	// Capabilities returns a list of the capabilities the provider supports.
	Capabilities() Capabilities

	// CreateVM will create a new microvm.
	CreateVM(ctx context.Context, input *CreateVMInput) (*CreateVMOutput, error)
	// StartVM will start a created microvm.
	StartVM(ctx context.Context, input *StartVMInput) (*StartVMOutput, error)
	// PauseVM will pause a started microvm.
	PauseVM(ctx context.CancelFunc, input *PauseVMInput) (*PauseVMOutput, error)
	// ResumeVM will resume a paused microvm.
	ResumeVM(ctx context.Context, input *ResumeVMInput) (*ResumeVMOutput, error)
	// StopVM will stop a paused or running microvm.
	StopVM(ctx context.Context, input *StopVMInput) (*StopVMOutput, error)
	// DeleteVM will delete a VM and its runtime state.
	DeleteVM(ctx context.Context, input *DeleteVMInput) (*DeleteVMOutput, error)

	// ListVMs will return a list of the microvms.
	ListVMs(ctx context.Context, input *ListVMsInput) (*ListVMsOutput, error)
}

// CreateVMInput is the input to CreateVM.
type CreateVMInput struct {
	// Spec is the specification of the microvm to create.
	Spec reignitev1.MicroVM `json:"spec"`
}

// CreateVMOutput is the output from CreateVM.
type CreateVMOutput struct {
	// VM is the details of the newly created microvm.
	VM *reignitev1.MicroVM
}

// StartVMInput is the input to StartVM.
type StartVMInput struct {
	// ID is the identifier of the microvm to start.
	ID string
}

// StartVMOutput is the putput of StartVM.
type StartVMOutput struct{}

// PauseVMInput is the input to PauseVM.
type PauseVMInput struct {
	// ID is the identifier of the microvm to pause.
	ID string
}

// PauseVMOutput is the output of PauseVM.
type PauseVMOutput struct{}

// ResumeVMInput is the input to ResumeVM.
type ResumeVMInput struct {
	// ID is the identifier of the microvm to resume.
	ID string
}

// ResumeVMOutput is the output of ResumeVM.
type ResumeVMOutput struct{}

// StopVMInput is the input to StopVM.
type StopVMInput struct {
	// ID is the identifier of the microvm to stop.
	ID string
}

// StopVMOutput is the output of StopVM.
type StopVMOutput struct{}

// DeleteVMInput is the input to DeleteVM.
type DeleteVMInput struct {
	// ID is the identifier of the microvm to delete.
	ID string
}

// DeleteVMOutput is the output of DeleteVM.
type DeleteVMOutput struct{}

// ListVMsInput is the input to ListVMs.
type ListVMsInput struct {
	// MaxResults is the maximum number of details to return.
	MaxResults int
}

// ListVMsOutput is the output of ListVMs.
type ListVMsOutput struct {
	// VMS is the list of VMs managed by this provider.
	VMS reignitev1.MicroVMList
}
