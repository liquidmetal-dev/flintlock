package firecracker

import (
	"context"
	"fmt"
	"os"

	firecracker "github.com/firecracker-microvm/firecracker-go-sdk"

	"github.com/weaveworks/reignite/core/models"
	"github.com/weaveworks/reignite/core/ports"
	"github.com/weaveworks/reignite/pkg/log"
)

// Config represents the configuration options for the Firecracker infrastructure.
type Config struct {
	// FirecrackerBin is the firecracker binary to use.
	FirecrackerBin string
	// SocketPath is the directory to use for the sockets.
	SocketPath string
}

// New creates a new instance of the firecracker microvm provider.
func New(cfg *Config) ports.MicroVMProvider {
	return &fcProvider{
		config: cfg,
	}
}

type fcProvider struct {
	config *Config
}

// Capabilities returns a list of the capabilities the Firecracker provider supports.
func (p *fcProvider) Capabilities() models.Capabilities {
	return models.Capabilities{models.MetadataServiceCapability}
}

// CreateVM will create a new microvm.
func (p *fcProvider) CreateVM(ctx context.Context, vm *models.MicroVM) (*models.MicroVM, error) {
	cfg, err := p.getConfig(vm)
	if err != nil {
		return nil, fmt.Errorf("getting firecracker configuration for machine: %w", err)
	}

	logger := log.GetLogger(ctx)
	opts := []firecracker.Opt{
		firecracker.WithLogger(logger),
	}

	// Only if not using the jailer
	builder := firecracker.VMCommandBuilder{}
	fcCmd := builder.
		WithBin(p.config.FirecrackerBin).
		WithStdout(os.Stdout). // TODO: change to file output
		WithStdin(os.Stdin).
		WithStderr(os.Stderr). // TODO: change to file output
		WithSocketPath("pathtosocket").
		Build(ctx)

	opts = append(opts, firecracker.WithProcessRunner(fcCmd))

	m, err := firecracker.NewMachine(ctx, *cfg, opts...)
	if err != nil {
		return nil, fmt.Errorf("creating new machine for %s: %w", vm.ID, err)
	}
	logger.Trace(m)

	return nil, nil
}

// StartVM will start a created microvm.
func (p *fcProvider) StartVM(ctx context.Context, id string) error {
	return errNotImplemeted
}

// PauseVM will pause a started microvm.
func (p *fcProvider) PauseVM(ctx context.Context, id string) error {
	return errNotImplemeted
}

// ResumeVM will resume a paused microvm.
func (p *fcProvider) ResumeVM(ctx context.Context, id string) error {
	return errNotImplemeted
}

// StopVM will stop a paused or running microvm.
func (p *fcProvider) StopVM(ctx context.Context, id string) error {
	return errNotImplemeted
}

// DeleteVM will delete a VM and its runtime state.
func (p *fcProvider) DeleteVM(ctx context.Context, id string) error {
	return errNotImplemeted
}

// ListVMs will return a list of the microvms.
func (p *fcProvider) ListVMs(ctx context.Context, count int) ([]*models.MicroVM, error) {
	return nil, errNotImplemeted
}
