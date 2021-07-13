package firecracker

import (
	"context"
	"fmt"
	"os"

	firecracker "github.com/firecracker-microvm/firecracker-go-sdk"

	"github.com/weaveworks/reignite/pkg/microvm"
)

const (
	ProviderName = "firecracker"
)

type Config struct {
	FirecrackerBin string
	SocketPath     string
}

// New creates a new instance of the firecracker microvm provider
func New(cfg *Config) (microvm.Provider, error) {
	return &fcProvider{
		config: cfg,
	}, nil
}

type fcProvider struct {
	config *Config
}

// Capabilities returns a list of the capabilities the Firecracker provider supports.
func (p *fcProvider) Capabilities() microvm.Capabilities {
	return microvm.Capabilities{microvm.MetadataServiceCapability}
}

// CreateVM will create a new microvm.
func (p *fcProvider) CreateVM(ctx context.Context, input *microvm.CreateVMInput) (*microvm.CreateVMOutput, error) {
	cfg, err := p.getConfig(&input.Spec)
	if err != nil {
		return nil, fmt.Errorf("getting firecracker configuration for machine: %w", err)
	}

	opts := []firecracker.Opt{
		//firecracker.WithLogger()
	}

	// Only if not using the jailer
	builder := firecracker.VMCommandBuilder{}
	fcCmd := builder.
		WithBin(p.config.FirecrackerBin).
		WithStdout(os.Stdout).
		WithStdin(os.Stdin).
		WithStderr(os.Stderr).
		WithSocketPath("pathtosocket").
		Build(ctx)

	opts = append(opts, firecracker.WithProcessRunner(fcCmd))

	m, err := firecracker.NewMachine(ctx, cfg, opts...)
	if err != nil {
		return nil, fmt.Errorf("creating new machine for %s: %w", input.Spec.Name, err)
	}
	fmt.Println(m)

	return nil, nil
}

// StartVM will start a created microvm.
func (p *fcProvider) StartVM(ctx context.Context, input *microvm.StartVMInput) (*microvm.StartVMOutput, error) {
	return nil, fmt.Errorf("not implemented")
}

// PauseVM will pause a started microvm.
func (p *fcProvider) PauseVM(ctx context.CancelFunc, input *microvm.PauseVMInput) (*microvm.PauseVMOutput, error) {
	return nil, fmt.Errorf("not implemented")
}

// ResumeVM will resume a paused microvm.
func (p *fcProvider) ResumeVM(ctx context.Context, input *microvm.ResumeVMInput) (*microvm.ResumeVMOutput, error) {
	return nil, fmt.Errorf("not implemented")
}

// StopVM will stop a paused or running microvm.
func (p *fcProvider) StopVM(ctx context.Context, input *microvm.StopVMInput) (*microvm.StopVMOutput, error) {
	return nil, fmt.Errorf("not implemented")
}

// DeleteVM will delete a VM and its runtime state.
func (p *fcProvider) DeleteVM(ctx context.Context, input *microvm.DeleteVMInput) (*microvm.DeleteVMOutput, error) {
	return nil, fmt.Errorf("not implemented")
}

// ListVMs will return a list of the microvms.
func (p *fcProvider) ListVMs(ctx context.Context, input *microvm.ListVMsInput) (*microvm.ListVMsOutput, error) {
	return nil, fmt.Errorf("not implemented")
}
