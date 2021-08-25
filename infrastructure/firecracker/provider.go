package firecracker

import (
	"context"
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"

	"github.com/firecracker-microvm/firecracker-go-sdk"
	fcmodels "github.com/firecracker-microvm/firecracker-go-sdk/client/models"

	"github.com/weaveworks/reignite/core/models"
	"github.com/weaveworks/reignite/core/ports"
	"github.com/weaveworks/reignite/pkg/log"
	"github.com/weaveworks/reignite/pkg/process"
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
	// APIConfig idnicates that te firecracker microvm should be configured using the API instead of file.
	APIConfig bool
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
func (p *fcProvider) Start(ctx context.Context, id string) error {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"service": "firecracker_microvm",
		"vmid":    id,
	})
	logger.Info("starting microvm")

	if !p.config.APIConfig {
		logger.Info("using firecracker configuration file, no explicit start required")

		return nil
	}

	vmid, err := models.NewVMIDFromString(id)
	if err != nil {
		return fmt.Errorf("parsing vmid: %w", err)
	}
	vmState := NewState(*vmid, p.config.StateRoot, p.fs)

	socketPath := vmState.SockPath()
	logger.Tracef("using socket %s", socketPath)

	client := firecracker.NewClient(socketPath, logger, true)
	_, err = client.CreateSyncAction(ctx, &fcmodels.InstanceActionInfo{
		ActionType: firecracker.String("InstanceStart"),
	})
	if err != nil {
		return fmt.Errorf("failed to create start action: %w", err)
	}

	logger.Info("started microvm")

	return nil
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
	return errNotImplemeted
}

// IsRunning returns true if the microvm is running.
func (p *fcProvider) IsRunning(ctx context.Context, id string) (bool, error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"service": "firecracker_microvm",
		"vmid":    id,
	})
	logger.Info("checking if microvm is running")

	vmid, err := models.NewVMIDFromString(id)
	if err != nil {
		return false, fmt.Errorf("parsing vmid: %w", err)
	}
	vmState := NewState(*vmid, p.config.StateRoot, p.fs)

	pidPath := vmState.PIDPath()
	exists, err := afero.Exists(p.fs, pidPath)
	if err != nil {
		return false, fmt.Errorf("checking pid file exists: %w", err)
	}
	if !exists {
		return false, nil
	}

	pid, err := vmState.PID()
	if err != nil {
		return false, fmt.Errorf("getting pid from file: %w", err)
	}

	processExists, err := process.Exists(pid)
	if err != nil {
		return false, fmt.Errorf("checking if firecracker process is running: %w", err)
	}
	if !processExists {
		return false, nil
	}

	// TODO: do we need to query via the sock interface?

	return true, nil
}
