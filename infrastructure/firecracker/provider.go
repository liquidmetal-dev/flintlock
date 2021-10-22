package firecracker

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/go-openapi/strfmt"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"

	"github.com/firecracker-microvm/firecracker-go-sdk"
	"github.com/firecracker-microvm/firecracker-go-sdk/client"
	fcmodels "github.com/firecracker-microvm/firecracker-go-sdk/client/models"
	"github.com/firecracker-microvm/firecracker-go-sdk/client/operations"

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
		logger.Debug("using firecracker configuration file, no explicit start required")

		return nil
	}

	running, err := p.IsRunning(ctx, id)
	if err != nil {
		return fmt.Errorf("checking if instance is running: %w", err)
	}
	if running {
		logger.Debug("instance is already running, not starting")

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
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"service": "firecracker_microvm",
		"vmid":    id,
	})
	logger.Info("deleting microvm")

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

	// This action will send the CTRL+ALT+DEL key sequence to the microVM. By
	// convention, this sequence has been used to trigger a soft reboot and, as
	// such, most Linux distributions perform an orderly shutdown and reset upon
	// receiving this keyboard input. Since Firecracker exits on CPU reset,
	// SendCtrlAltDel can be used to trigger a clean shutdown of the microVM.
	//
	// Source: https://github.com/firecracker-microvm/firecracker/blob/main/docs/api_requests/actions.md#intel-and-amd-only-sendctrlaltdel
	_, err = client.CreateSyncAction(ctx, &fcmodels.InstanceActionInfo{
		ActionType: firecracker.String("SendCtrlAltDel"),
	})
	if err != nil {
		// What errors do we want to ignore?
		// Example:
		// * net/url.Error happens if the VM is not running or the socket file
		//   is not there, so we can delete the VM.
		if errors.Is(err, &url.Error{}) {
			logger.Info("microvm is not running")
		} else {
			return fmt.Errorf("failed to create halt action: %w", err)
		}
	}

	// It's strange to call it delete, it terminates the vm, but by the nature of
	// firecracker, if it's terminated, it's not there anymore, only the
	// resources we created before, but we have steps for them.

	logger.Info("deleted microvm")

	return nil
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

	socketPath := vmState.SockPath()
	logger.Tracef("using socket %s", socketPath)

	info, err := p.getInstanceInfo(socketPath, logger)
	if err != nil {
		return false, fmt.Errorf("getting instance info: %w", err)
	}

	if *info.State == string(InstanceStateStarted) {
		return true, nil
	}

	return false, nil
}

func (p *fcProvider) getInstanceInfo(socketPath string, logger *logrus.Entry) (*fcmodels.InstanceInfo, error) {
	httpClient := client.NewHTTPClient(strfmt.NewFormats())

	transport := firecracker.NewUnixSocketTransport(socketPath, logger, true)
	httpClient.SetTransport(transport)

	resp, err := httpClient.Operations.DescribeInstance(operations.NewDescribeInstanceParams())
	if err != nil {
		return nil, fmt.Errorf("describing firecracker instance: %w", err)
	}

	return resp.Payload, nil
}
