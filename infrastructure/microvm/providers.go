package microvm

import (
	"errors"

	"github.com/spf13/afero"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/infrastructure/microvm/cloudhypervisor"
	"github.com/liquidmetal-dev/flintlock/infrastructure/microvm/firecracker"
	"github.com/liquidmetal-dev/flintlock/internal/config"
)

var errUnknownProvider = errors.New("unknown provider")

// VolumeCapabilities are the volume modes the host accepts, as the startup
// probes found them. Every provider advertises them.
type VolumeCapabilities models.Capabilities

// New will create a new instance of a microvm service from the supplied name.
func New(name string,
	cfg *config.Config,
	volumeCaps VolumeCapabilities,
	networkSvc ports.NetworkService,
	diskSvc ports.DiskService,
	fs afero.Fs,
) (ports.MicroVMService, error) {
	switch name {
	case firecracker.ProviderName:
		return firecracker.New(firecrackerConfig(cfg, volumeCaps), networkSvc, fs), nil
	case cloudhypervisor.ProviderName:
		return cloudhypervisor.New(cloudHypervisorConfig(cfg, volumeCaps), networkSvc, diskSvc, fs), nil
	default:
		return nil, errUnknownProvider
	}
}

// NewFromConfig will create instances of the vm providers based on the config.
func NewFromConfig(cfg *config.Config,
	volumeCaps VolumeCapabilities,
	networkSvc ports.NetworkService,
	diskSvc ports.DiskService,
	fs afero.Fs,
) (map[string]ports.MicroVMService, error) {
	providers := map[string]ports.MicroVMService{}

	if cfg.CloudHypervisorBin != "" {
		providers[cloudhypervisor.ProviderName] = cloudhypervisor.New(
			cloudHypervisorConfig(cfg, volumeCaps), networkSvc, diskSvc, fs,
		)
	}
	if cfg.FirecrackerBin != "" {
		providers[firecracker.ProviderName] = firecracker.New(firecrackerConfig(cfg, volumeCaps), networkSvc, fs)
	}

	if len(providers) == 0 {
		return nil, errors.New("you must enable at least 1 microvm provider")
	}

	return providers, nil
}

func GetProviderNames() []string {
	return []string{
		firecracker.ProviderName,
		cloudhypervisor.ProviderName,
	}
}

func firecrackerConfig(cfg *config.Config, volumeCaps VolumeCapabilities) *firecracker.Config {
	return &firecracker.Config{
		FirecrackerBin:     cfg.FirecrackerBin,
		RunDetached:        cfg.FirecrackerDetatch,
		StateRoot:          cfg.StateRootDir + "/vm",
		SocketDir:          cfg.SocketDir,
		VolumeCapabilities: models.Capabilities(volumeCaps),
	}
}

func cloudHypervisorConfig(cfg *config.Config, volumeCaps VolumeCapabilities) *cloudhypervisor.Config {
	return &cloudhypervisor.Config{
		CloudHypervisorBin: cfg.CloudHypervisorBin,
		RunDetached:        cfg.CloudHypervisorDetatch,
		StateRoot:          cfg.StateRootDir + "/vm",
		SocketDir:          cfg.SocketDir,
		VolumeCapabilities: models.Capabilities(volumeCaps),
	}
}
