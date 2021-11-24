package firecracker

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"

	"github.com/weaveworks/flintlock/core/models"
	"github.com/weaveworks/flintlock/core/ports"
	"github.com/weaveworks/flintlock/pkg/log"
	"github.com/weaveworks/flintlock/pkg/process"
)

var errNotImplemeted = errors.New("not implemented")

// Config represents the configuration options for the Firecracker infrastructure.
type Config struct {
	// FirecrackerBin is the firecracker binary to use.
	FirecrackerBin string
	// StateRoot is the folder to store any required firecracker state (i.e. socks, pid, log files).
	StateRoot string
	// RunDetached indicates that the firecracker processes should be run detached (a.k.a daemon) from the parent process.
	RunDetached bool
}

// New creates a new instance of the firecracker microvm provider.
func New(cfg *Config, networkSvc ports.NetworkService, fs afero.Fs) ports.MicroVMService {
	return &fcProvider{
		config:     cfg,
		networkSvc: networkSvc,
		fs:         fs,
	}
}

type fcProvider struct {
	config *Config

	networkSvc ports.NetworkService
	fs         afero.Fs
}

// Capabilities returns a list of the capabilities the Firecracker provider supports.
func (p *fcProvider) Capabilities() models.Capabilities {
	return models.Capabilities{models.MetadataServiceCapability}
}

// StartVM will start a created microvm.
// With configuration file, we don't really have start.  A separate Start and
// Create steps is a good idea, but the right now steps are still coupled with
// MicroVM.
//
// The two options are:
//  A) Merge Create and Start steps in global scope.
//  B) Let Start to just call Create.
//
// Without heavy refactoring, option A seems more logical and let us keep the
// separate Create and Start steps.  If we merge them togther, we may face
// issues when we try to add new MicroVM providers, so that way we would work
// twice on the same thing, now remove them and then add it back and make a
// separation here, like option B.
func (p *fcProvider) Start(ctx context.Context, vm *models.MicroVM) error {
	return p.Create(ctx, vm)
}

// Pause will pause a started microvm.
func (p *fcProvider) Pause(ctx context.Context, id string) error {
	return errNotImplemeted
}

// Resume will resume a paused microvm.
func (p *fcProvider) Resume(ctx context.Context, id string) error {
	return errNotImplemeted
}

// Stop will stop a paused or running microvm.
func (p *fcProvider) Stop(ctx context.Context, id string) error {
	return errNotImplemeted
}

// Delete will delete a VM and its runtime state.
func (p *fcProvider) Delete(ctx context.Context, id string) error {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"service": "firecracker_microvm",
		"vmid":    id,
	})
	logger.Info("deleting microvm")

	vmid, err := models.NewVMIDFromString(id)
	if err != nil {
		return fmt.Errorf("parsing vmid: %w", err)
	}

	vmState := NewState(*vmid, p.config.StateRoot, p.fs)

	pid, pidErr := vmState.PID()
	if pidErr != nil {
		return fmt.Errorf("unable to get PID: %w", err)
	}

	logger.Infof("sending SIGINT to %d", pid)

	if sigErr := process.SendSignal(pid, os.Interrupt); sigErr != nil {
		return fmt.Errorf("failed to terminate with SIGINT: %w", err)
	}

	logger.Info("deleted microvm")

	return nil
}

// State returns the state of a Firecracker microvm.
func (p *fcProvider) State(ctx context.Context, id string) (ports.MicroVMState, error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"service": "firecracker_microvm",
		"vmid":    id,
	})
	logger.Info("checking state of microvm")

	vmid, err := models.NewVMIDFromString(id)
	if err != nil {
		return ports.MicroVMStateUnknown, fmt.Errorf("parsing vmid: %w", err)
	}

	vmState := NewState(*vmid, p.config.StateRoot, p.fs)
	pidPath := vmState.PIDPath()

	exists, err := afero.Exists(p.fs, pidPath)
	if err != nil {
		return ports.MicroVMStateUnknown, fmt.Errorf("checking pid file exists: %w", err)
	}

	if !exists {
		return ports.MicroVMStatePending, nil
	}

	pid, err := vmState.PID()
	if err != nil {
		return ports.MicroVMStateUnknown, fmt.Errorf("getting pid from file: %w", err)
	}

	processExists, err := process.Exists(pid)
	if err != nil {
		return ports.MicroVMStateUnknown, fmt.Errorf("checking if firecracker process is running: %w", err)
	}

	if !processExists {
		return ports.MicroVMStatePending, nil
	}

	return ports.MicroVMStateRunning, nil
}
