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

// SnapshotCommandUseCases is the interface for uses cases that are actions (a.k.a commands)
// against a snapshot.
type SnapshotCommandUseCases interface {
	// CreateSnapshot is a use case for requesting a snapshot of a microvm. It returns the
	// pending snapshot record; the capture runs afterwards.
	CreateSnapshot(ctx context.Context, req models.CreateSnapshotRequest) (*models.Snapshot, error)
	// DeleteSnapshot is a use case for deleting a snapshot.
	DeleteSnapshot(ctx context.Context, ref models.SnapshotRef) error
}

// SnapshotQueryUseCases is the interface for uses cases that are queries for snapshots.
type SnapshotQueryUseCases interface {
	// GetSnapshot is a use case for getting details of a specific snapshot.
	GetSnapshot(ctx context.Context, ref models.SnapshotRef) (*models.Snapshot, error)
	// GetAllSnapshots is a use case for getting details of the snapshots that match the options.
	GetAllSnapshots(ctx context.Context, options SnapshotListOptions) ([]*models.Snapshot, error)
}

// ReconcileMicroVMsUseCase is the interface for use cases that are related to reconciling microvms.
type ReconcileMicroVMsUseCase interface {
	// ReconcileMicroVM is a use case for reconciling a specific microvm.
	ReconcileMicroVM(ctx context.Context, vmid models.VMID) error
}
