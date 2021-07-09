package provider

import "context"

type MicrovmProvider interface {
	Name() string

	SetupProvider(ctx context.Context, runtime *RuntimeOptions) error

	CreateVM(ctx context.Context, input *CreateVMInput) (*CreateVMOutput, error)
	StartVM(ctx context.Context, input *StartVMInput) (*StartVMOutput, error)
	PauseVM(ctx context.CancelFunc, input *PauseVMInput) (*PauseVMOutput, error)
	ResumeVM(ctx context.Context, input *ResumeVMInput) (*ResumeVMOutput, error)
	StopVM(ctx context.Context, input *StopVMInput) (*StopVMOutput, error)
	DeleteVM(ctx context.Context, input *DeleteVMInput) (*DeleteVMOutput, error)

	ListVMs(ctx context.Context, input *ListVMsInput) (*ListVMsOutput, error)
}

type CreateVMInput struct {
	ID   string // optional
	Spec VirtualMachineSpec
}

type CreateVMOutput struct {
	VM *VirtualMachine
}

type StartVMInput struct {
	ID string
}

type StartVMOutput struct{}

type PauseVMInput struct {
	ID string
}

type PauseVMOutput struct{}

type ResumeVMInput struct {
	ID string
}

type ResumeVMOutput struct{}

type StopVMInput struct {
	ID string
}

type StopVMOutput struct{}

type DeleteVMInput struct {
	ID string
}

type DeleteVMOutput struct{}

type ListVMsInput struct {
	MaxResults int // Optional, max results
}

type ListVMsOutput struct {
	VMS []VirtualMachine
}
