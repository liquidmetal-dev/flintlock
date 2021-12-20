package ports

import (
	"context"

	"github.com/weaveworks/flintlock/core/models"
)

// MicroVMCommandUseCases is the interface for uses cases that are actions (a.k.a commands) against a microvm.
type MicroVMCommandUseCases interface {
	// CreateMicroVM is a use case for creating a microvm.
	CreateMicroVM(ctx context.Context, mvm *models.MicroVM) (*models.MicroVM, error)
	// DeleteMicroVM is a use case for deleting a microvm.
	DeleteMicroVM(ctx context.Context, id, namespace string) error
}

// MicroVMQueryUseCases is the interface for uses cases that are queries for microvms.
type MicroVMQueryUseCases interface {
	// GetMicroVM is a use case for getting details of a specific microvm.
	GetMicroVM(ctx context.Context, id, namespace string) (*models.MicroVM, error)
	// GetAllMicroVM is a use case for getting details of all microvms in a given namespace.
	GetAllMicroVM(ctx context.Context, namespace string) ([]*models.MicroVM, error)
	// GetMicroVMStatus gets the status of a microvm based on the spec and running state of the microvm.
	GetMicroVMStatus(ctx context.Context, id, namespace string) (models.Status, error)
}

// ReconcileMicroVMsUseCase is the interface for use cases that are related to reconciling microvms.
type ReconcileMicroVMsUseCase interface {
	// ReconcileMicroVM is a use case for reconciling a specific microvm.
	ReconcileMicroVM(ctx context.Context, id, namespace string) error
	// ResyncMicroVMs is used to resync the microvms. If a namespace is supplied then it will
	// resync only the microvms in that namespaces.
	ResyncMicroVMs(ctx context.Context, namespace string) error
}
