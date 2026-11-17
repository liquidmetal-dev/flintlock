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
	// PauseMicroVM is a use case for pausing a running microvm.
	PauseMicroVM(ctx context.Context, uid string) error
	// ResumeMicroVM is a use case for resuming a paused microvm.
	ResumeMicroVM(ctx context.Context, uid string) error
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

// SnapshotCaptureUseCase is the interface for the use case that captures a snapshot.
type SnapshotCaptureUseCase interface {
	// CaptureSnapshot captures the snapshot of the record from its source microvm.
	// It does not change the phase of the record.
	CaptureSnapshot(ctx context.Context, snapshot *models.Snapshot) error
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
