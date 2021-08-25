package mock

//go:generate ../../hack/tools/bin/mockgen -destination mock.go -package mock github.com/weaveworks/reignite/core/ports MicroVMService,MicroVMRepository,EventService,IDService,ImageService,ReconcileMicroVMsUseCase,NetworkService
