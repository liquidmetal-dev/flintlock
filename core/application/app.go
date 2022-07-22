package application

import (
	"github.com/weaveworks-liquidmetal/flintlock/core/ports"
)

// App is the interface for the core application. In the future this could be split
// into separate command, query and reconcile services.
type App interface {
	ports.MicroVMCommandUseCases
	ports.MicroVMQueryUseCases
	ports.ReconcileMicroVMsUseCase
}

func New(cfg *Config, ports *ports.Collection) App {
	return &app{
		cfg:   cfg,
		ports: ports,
	}
}

type app struct {
	cfg   *Config
	ports *ports.Collection
}

type Config struct {
	UseCloudInitVolume bool
	RootStateDir       string
	MaximumRetry       int
}
