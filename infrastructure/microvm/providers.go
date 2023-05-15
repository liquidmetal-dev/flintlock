package microvm

import (
	"errors"
	"fmt"

	"github.com/spf13/afero"

	"github.com/weaveworks-liquidmetal/flintlock/core/ports"
	"github.com/weaveworks-liquidmetal/flintlock/infrastructure/microvm/cloudhypervisor"
	"github.com/weaveworks-liquidmetal/flintlock/infrastructure/microvm/firecracker"
	"github.com/weaveworks-liquidmetal/flintlock/internal/config"
)

var (
	errUnknownProvider = errors.New("unknown provider")
)

// New will create a new instance of a microvm service from the supplied name.
func New(name string, cfg *config.Config, networkSvc ports.NetworkService, diskSvc ports.DiskService, fs afero.Fs) (ports.MicroVMService, error) {
	switch name {
	case firecracker.ProviderName:
		return firecracker.New(firecrackerConfig(cfg), networkSvc, fs), nil
	case cloudhypervisor.ProviderName:
		return cloudhypervisor.New(cloudHypervisorConfig(cfg), networkSvc, diskSvc, fs), nil
	default:
		return nil, errUnknownProvider
	}
}

// NewFromConfig will create instances of the vm providers based on the config.
func NewFromConfig(cfg *config.Config, networkSvc ports.NetworkService, diskSvc ports.DiskService, fs afero.Fs) (map[string]ports.MicroVMService, error) {
	providers := map[string]ports.MicroVMService{}

	if cfg.CloudHypervisorBin != "" {
		providers[cloudhypervisor.ProviderName] = cloudhypervisor.New(cloudHypervisorConfig(cfg), networkSvc, diskSvc, fs)
	}
	if cfg.FirecrackerBin != "" {
		providers[firecracker.ProviderName] = firecracker.New(firecrackerConfig(cfg), networkSvc, fs)
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

func firecrackerConfig(cfg *config.Config) *firecracker.Config {
	return &firecracker.Config{
		FirecrackerBin: cfg.FirecrackerBin,
		RunDetached:    cfg.FirecrackerDetatch,
		StateRoot:      fmt.Sprintf("%s/vm", cfg.StateRootDir),
	}
}

func cloudHypervisorConfig(cfg *config.Config) *cloudhypervisor.Config {
	return &cloudhypervisor.Config{
		CloudHypervisorBin: cfg.CloudHypervisorBin,
		RunDetached:        cfg.CloudHypervisorDetatch,
		StateRoot:          fmt.Sprintf("%s/vm", cfg.StateRootDir),
	}
}
