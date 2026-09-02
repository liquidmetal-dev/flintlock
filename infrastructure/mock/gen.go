package mock

//go:generate mockgen -destination ports.go -package mock github.com/liquidmetal-dev/flintlock/core/ports MicroVMService,MicroVMRepository,EventService,IDService,ImageService,ReconcileMicroVMsUseCase,NetworkService,MicroVMCommandUseCases,MicroVMQueryUseCases
//go:generate mockgen -destination containerd.go -package mock github.com/liquidmetal-dev/flintlock/infrastructure/containerd Client
//go:generate mockgen -destination ext_containerd_leases.go -package mock github.com/containerd/containerd/leases Manager
//go:generate mockgen -destination ext_containerd_snapshots.go -package mock github.com/containerd/containerd/snapshots Snapshotter
//go:generate mockgen -destination ext_containerd.go -package mock github.com/containerd/containerd Image
