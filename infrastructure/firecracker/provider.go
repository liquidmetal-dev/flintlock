package firecracker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/weaveworks-liquidmetal/flintlock/core/models"
	"github.com/weaveworks-liquidmetal/flintlock/core/ports"
	"github.com/weaveworks-liquidmetal/flintlock/pkg/log"
	"github.com/weaveworks-liquidmetal/flintlock/pkg/process"
	tailor "github.com/yitsushi/file-tailor"
)

// Config represents the configuration options for the Firecracker infrastructure.
type Config struct {
	// FirecrackerBin is the firecracker binary to use.
	FirecrackerBin string
	// StateRoot is the folder to store any required firecracker state (i.e. socks, pid, log files).
	StateRoot string
	// RunDetached indicates that the firecracker processes should be run detached (a.k.a daemon) from the parent process.
	RunDetached bool
	// DeleteVMTimeout is the timeout to wait for the microvm to be deleted.
	DeleteVMTimeout time.Duration
	// CloudInitFromMMDS if true will setup cloud-init to run from the MMDS instead of a volume.
	CloudInitFromMMDS bool
}

// New creates a new instance of the firecracker microvm provider.
func New(cfg *Config, networkSvc ports.NetworkService, fs afero.Fs) ports.MicroVMService {
	return &fcProvider{
		config:          cfg,
		networkSvc:      networkSvc,
		fs:              fs,
		deleteVMTimeout: cfg.DeleteVMTimeout,
	}
}

type fcProvider struct {
	config *Config

	networkSvc      ports.NetworkService
	fs              afero.Fs
	deleteVMTimeout time.Duration
}

// Capabilities returns a list of the capabilities the Firecracker provider supports.
func (p *fcProvider) Capabilities() models.Capabilities {
	return models.Capabilities{models.MetadataServiceCapability}
}

// Start will start a created microvm.
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

// Stop will stop a running microvm.
func (p *fcProvider) Delete(ctx context.Context, vm *models.MicroVM) error {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"service": "firecracker_microvm",
		"vmid":    vm.ID,
	})
	logger.Info("deleting microvm")

	vmState := NewState(vm.Status.RuntimeStateDir, p.fs)

	pid, pidErr := vmState.PID()
	if pidErr != nil {
		return fmt.Errorf("unable to get PID: %w", pidErr)
	}

	logger.Infof("sending SIGHUP to %d", pid)

	if sigErr := process.SendSignal(pid, syscall.SIGHUP); sigErr != nil {
		return fmt.Errorf("failed to terminate with SIGHUP: %w", sigErr)
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, p.deleteVMTimeout)
	defer cancel()

	// Make sure the microVM is stopped.
	if err := process.WaitWithContext(ctxTimeout, pid); err != nil {
		return fmt.Errorf("failed to wait for pid %d: %w", pid, err)
	}

	logger.Info("deleted microvm")

	return nil
}

// State returns the state of a Firecracker microvm.
func (p *fcProvider) State(ctx context.Context, vm *models.MicroVM) (ports.MicroVMState, error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"service": "firecracker_microvm",
		"vmid":    vm.ID,
	})
	logger.Info("checking state of microvm")

	vmState := NewState(vm.Status.RuntimeStateDir, p.fs)
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

func (p *fcProvider) Metrics(ctx context.Context, vm *models.MicroVM) (ports.MachineMetrics, error) {
	machineMetrics := MachineMetrics{
		Namespace:   vm.ID.Namespace(),
		MachineName: vm.ID.Name(),
		MachineUID:  vm.ID.UID(),
		Data:        Metrics{},
	}

	vmState := NewState(p.config.StateRoot, p.fs)

	file, err := os.Open(vmState.MetricsPath())
	if err != nil {
		return machineMetrics, fmt.Errorf("unable to open metrics file: %w", err)
	}

	defer file.Close()

	content, err := tailor.Tail(file, 1)
	if err != nil {
		return machineMetrics, fmt.Errorf("unable to read the last line of the metrics file: %w", err)
	}

	// It can throw an error, but we don't care.
	// For example the utc_timestamp_ms field is in the root of the metrics JSON,
	// and it does not follow the map[string]string pattern, but we don't care
	// about that value.
	_ = json.Unmarshal(content, &machineMetrics.Data)

	return machineMetrics, nil
}
