package cloudhypervisor

import (
	"context"
	"fmt"
	"time"

	"github.com/liquidmetal-dev/flintlock/pkg/cloudhypervisor"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"

	cerrs "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/internal/config"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
	"github.com/liquidmetal-dev/flintlock/pkg/process"
)

const (
	ProviderName = "cloudhypervisor"
)

// Config represents the configuration options for the Cloud Hypervisor infrastructure.
type Config struct {
	// CloudHypervisorBin is the Cloud Hypervisor binary to use.
	CloudHypervisorBin string
	// VirtioFSBin is the virtiofs binary to use.
	VirtioFSBin string
	// StateRoot is the folder to store any required state (i.e. socks, pid, log files).
	StateRoot string
	// RunDetached indicates that the cloud hypervisor processes
	// should be run detached (a.k.a daemon) from the parent process.
	RunDetached bool
	// DeleteVMTimeout is the timeout to wait for the microvm to be deleted.
	DeleteVMTimeout time.Duration
}

func New(cfg *Config, networkSvc ports.NetworkService, ds ports.DiskService, fs afero.Fs) ports.MicroVMService {
	return &provider{
		config:          cfg,
		networkSvc:      networkSvc,
		fs:              fs,
		diskSvc:         ds,
		deleteVMTimeout: cfg.DeleteVMTimeout,
	}
}

type provider struct {
	config *Config

	networkSvc      ports.NetworkService
	diskSvc         ports.DiskService
	fs              afero.Fs
	deleteVMTimeout time.Duration
}

// Capabilities returns a list of the capabilities the provider supports.
func (p *provider) Capabilities() models.Capabilities {
	return []models.Capability{models.AutoStartCapability, models.MacvtapCapability, models.VirtioFSCapability}
}

// Start will start a created microvm.
func (p *provider) Start(_ context.Context, _ *models.MicroVM) error {
	return cerrs.NewNotSupported("start")
}

// State returns the state of a microvm.
func (p *provider) State(ctx context.Context, id string) (ports.MicroVMState, error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"service": "cloudhypervisor_microvm",
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
		return ports.MicroVMStateUnknown, fmt.Errorf("checking if cloudhypervisor process is running: %w", err)
	}

	if !processExists {
		return ports.MicroVMStatePending, nil
	}

	sockExists, err := afero.Exists(p.fs, vmState.SockPath())
	if err != nil {
		return ports.MicroVMStateUnknown, fmt.Errorf("checking sock file exists: %w", err)
	}

	if !sockExists {
		return ports.MicroVMStatePending, nil
	}

	chClient := cloudhypervisor.New(vmState.SockPath())

	vmInfo, err := chClient.Info(ctx)
	if err != nil {
		return ports.MicroVMStateUnknown, fmt.Errorf("querying cloud-hypervisor for info: %w", err)
	}

	// NOTE: we can support paused/unpause in the future
	switch vmInfo.State {
	case cloudhypervisor.VMStateRunning:
		return ports.MicroVMStateRunning, nil
	case cloudhypervisor.VMStateCreated:
		return ports.MicroVMStatePending, nil
	default:
		return ports.MicroVMStateUnknown, fmt.Errorf("cloud-hypervisor in an unsupported state: %s", vmInfo.State)
	}
}

// DefaultKernelCmdLine is the default recommended kernel parameter list.
//
// console=ttyS0   [KLN] Output console device and options
// reboot=k        [KNL] reboot_type=kbd
// panic=1         [KNL] Kernel behaviour on panic: delay <timeout>
//
//	timeout > 0: seconds before rebooting
//	timeout = 0: wait forever
//	timeout < 0: reboot immediately
//
// i8042.noaux     [HW]  Don't check for auxiliary (== mouse) port
// i8042.nomux     [HW]  Don't check presence of an active multiplexing
//
//	controller
//
// i8042.nopnp     [HW]  Don't use ACPIPnP / PnPBIOS to discover KBD/AUX
//
//	controllers
//
// i8042.dumbkbd   [HW]  Pretend that controller can only read data from
//
//	keyboard and cannot control its state
//	(Don't attempt to blink the leds)
//
// Read more:
// https://www.kernel.org/doc/html/v5.15/admin-guide/kernel-parameters.html
func DefaultKernelCmdLine() config.KernelCmdLine {
	return config.KernelCmdLine{
		"console": "hvc0",
		"root":    "/dev/vda",
		"rw":      "",
		"reboot":  "k",
		"panic":   "1",
		// "i8042.noaux":   "",
		// "i8042.nomux":   "",
		// "i8042.nopnp":   "",
		// "i8042.dumbkbd": "",
	}
}
