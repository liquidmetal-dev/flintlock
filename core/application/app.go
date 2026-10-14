package application

import (
	"github.com/liquidmetal-dev/flintlock/core/ports"
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
	RootStateDir    string
	SocketDir       string
	MaximumRetry    int
	DefaultProvider string
	// OverlayInitrdImage is the OCI image of the initrd that boots an overlay
	// root when the spec names none. An empty value means the daemon has no
	// default, and a spec that needs one is rejected.
	OverlayInitrdImage string
}
