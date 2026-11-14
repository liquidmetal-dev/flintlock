package ports

import (
	"context"

	"github.com/liquidmetal-dev/flintlock/core/models"
)

// MicroVMCommandUseCases is the interface for uses cases that are actions (a.k.a commands) against a microvm.
type MicroVMCommandUseCases interface {
	// CreateMicroVM is a use case for creating a microvm.
	CreateMicroVM(ctx context.Context, mvm *models.MicroVM) (*models.MicroVM, error)
	// DeleteMicroVM is a use case for deleting a microvm.
	DeleteMicroVM(ctx context.Context, vmid string) error
}

// MicroVMQueryUseCases is the interface for uses cases that are queries for microvms.
type MicroVMQueryUseCases interface {
	// GetMicroVM is a use case for getting details of a specific microvm.
	GetMicroVM(ctx context.Context, vmid string) (*models.MicroVM, error)
	// GetAllMicroVM is a use case for getting details of all microvms in a given namespace.
	GetAllMicroVM(ctx context.Context, query models.ListMicroVMQuery) ([]*models.MicroVM, error)
}

// ReconcileMicroVMsUseCase is the interface for use cases that are related to reconciling microvms.
type ReconcileMicroVMsUseCase interface {
	// ReconcileMicroVM is a use case for reconciling a specific microvm.
	ReconcileMicroVM(ctx context.Context, vmid models.VMID) error
}
