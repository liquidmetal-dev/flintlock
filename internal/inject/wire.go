//go:build wireinject
// +build wireinject

package inject

import (
	"time"

	"github.com/google/wire"
	"github.com/spf13/afero"

	"github.com/weaveworks-liquidmetal/flintlock/core/application"
	"github.com/weaveworks-liquidmetal/flintlock/core/ports"
	"github.com/weaveworks-liquidmetal/flintlock/infrastructure/containerd"
	"github.com/weaveworks-liquidmetal/flintlock/infrastructure/controllers"
	microvmgrpc "github.com/weaveworks-liquidmetal/flintlock/infrastructure/grpc"
	"github.com/weaveworks-liquidmetal/flintlock/infrastructure/microvm"
	"github.com/weaveworks-liquidmetal/flintlock/infrastructure/network"
	"github.com/weaveworks-liquidmetal/flintlock/infrastructure/ulid"
	"github.com/weaveworks-liquidmetal/flintlock/internal/config"
	"github.com/weaveworks-liquidmetal/flintlock/pkg/defaults"
)

func InitializePorts(microvmProviderName string, cfg *config.Config) (*ports.Collection, error) {
	wire.Build(containerd.NewEventService,
		containerd.NewImageService,
		containerd.NewMicroVMRepo,
		ulid.New,
		microvm.New,
		network.New,
		appPorts,
		containerdConfig,
		networkConfig,
		afero.NewOsFs)

	return nil, nil
}

func InitializeApp(cfg *config.Config, ports *ports.Collection) application.App {
	wire.Build(application.New, appConfig)

	return nil
}

func InializeController(app application.App, ports *ports.Collection) *controllers.MicroVMController {
	wire.Build(controllers.New, eventSvcFromScope, reconcileUCFromApp, queryUCFromApp)

	return nil
}

func InitializeGRPCServer(app application.App) ports.MicroVMGRPCService {
	wire.Build(microvmgrpc.NewServer, queryUCFromApp, commandUCFromApp)

	return nil
}

func containerdConfig(cfg *config.Config) *containerd.Config {
	return &containerd.Config{
		SnapshotterKernel: cfg.CtrSnapshotterKernel,
		SnapshotterVolume: defaults.ContainerdVolumeSnapshotter,
		SocketPath:        cfg.CtrSocketPath,
		Namespace:         cfg.CtrNamespace,
	}
}

func networkConfig(cfg *config.Config) *network.Config {
	return &network.Config{
		ParentDeviceName: cfg.ParentIface,
		BridgeName:       cfg.BridgeName,
	}
}

func appConfig(cfg *config.Config) *application.Config {
	return &application.Config{
		RootStateDir: cfg.StateRootDir,
		MaximumRetry: cfg.MaximumRetry,
	}
}

func appPorts(repo ports.MicroVMRepository, prov ports.MicroVMService, es ports.EventService, is ports.IDService, ns ports.NetworkService, ims ports.ImageService, fs afero.Fs) *ports.Collection {
	return &ports.Collection{
		Repo:              repo,
		Provider:          prov,
		EventService:      es,
		IdentifierService: is,
		NetworkService:    ns,
		ImageService:      ims,
		FileSystem:        fs,
		Clock:             time.Now,
	}
}

func eventSvcFromScope(ports *ports.Collection) ports.EventService {
	return ports.EventService
}

func reconcileUCFromApp(app application.App) ports.ReconcileMicroVMsUseCase {
	return app
}

func queryUCFromApp(app application.App) ports.MicroVMQueryUseCases {
	return app
}

func commandUCFromApp(app application.App) ports.MicroVMCommandUseCases {
	return app
}
