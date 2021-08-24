package application

import "github.com/weaveworks/reignite/core/ports"

// AppS is the interface for the core application. In the future this could be split
// into separate command, query and reconcile services.
type App interface {
	ports.MicroVMCommandUseCases
	ports.MicroVMQueryUseCases
	ports.ReconcileMicroVMsUseCase
}

func New(repo ports.MicroVMRepository, eventSvc ports.EventService, idSvc ports.IDService, mvmProvider ports.MicroVMProvider) App {
	return &app{
		repo:     repo,
		eventSvc: eventSvc,
		idSvc:    idSvc,
		provider: mvmProvider,
	}
}

type app struct {
	repo     ports.MicroVMRepository
	eventSvc ports.EventService
	idSvc    ports.IDService
	provider ports.MicroVMProvider
}
