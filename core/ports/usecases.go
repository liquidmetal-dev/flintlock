package ports

import (
	"context"

	"github.com/weaveworks/reignite/core/models"
)

type MicroVMCommandUseCases interface {
	CreateMicroVM(ctx context.Context, mvm *models.MicroVM) (*models.MicroVM, error)
	UpdateMicroVM(ctx context.Context, mvm *models.MicroVM) (*models.MicroVM, error)
	DeleteMicroVM(ctx context.Context, id, namespace string) error
}

type MicroVMQueryUseCases interface {
	GetMicroVM(ctx context.Context, id, namespace string) (*models.MicroVM, error)
	GetAllMicroVM(ctx context.Context, namespace string) ([]*models.MicroVM, error)
}

type ReconcileMicroVMsUseCase interface {
	ReconcileMicroVMs(ctx context.Context) error
}
