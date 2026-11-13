package ports

import (
	"context"

	"github.com/liquidmetal-dev/flintlock/core/models"
)

type RepositoryGetOptions struct {
	Name      string
	Namespace string
	Version   string
	UID       string
}

// MicroVMRepository is the port definition for a microvm repository.
type MicroVMRepository interface {
	// Save will save the supplied microvm spec.
	Save(ctx context.Context, microvm *models.MicroVM) (*models.MicroVM, error)
	// Delete will delete the supplied microvm.
	Delete(ctx context.Context, microvm *models.MicroVM) error
	// Get will get the microvm spec with the given name/namespace.
	Get(ctx context.Context, options RepositoryGetOptions) (*models.MicroVM, error)
	// GetAll will get a list of microvm details. If namespace is an empty string all
	// details of microvms will be returned.
	GetAll(ctx context.Context, query models.ListMicroVMQuery) ([]*models.MicroVM, error)
	// Exists checks to see if the microvm spec exists in the repo.
	Exists(ctx context.Context, vmid models.VMID) (bool, error)
	// ReleaseLease will release the supplied lease.
	ReleaseLease(ctx context.Context, microvm *models.MicroVM) error
}

// SnapshotListOptions filters the snapshots that SnapshotRepository.List returns.
// An empty field matches every snapshot.
type SnapshotListOptions struct {
	// Namespace matches the namespace of the snapshot.
	Namespace string
	// SourceUID matches the uid of the microvm the snapshot was taken from.
	SourceUID string
	// Labels matches snapshots that have every one of these labels.
	Labels map[string]string
}

// SnapshotRepository is the port definition for the store of snapshots on the host.
// Get and List do not return a deleted snapshot, but its files can stay on the host
// until no restored microvm references them.
type SnapshotRepository interface {
	// Create makes the directory and a pending record for the snapshot. It sets the
	// uid, name, namespace and creation time when they are empty. It returns an
	// already exists error when the name is taken in the namespace.
	Create(ctx context.Context, snapshot *models.Snapshot) error
	// Get returns the snapshot with the uid of ref, or with its name and namespace.
	Get(ctx context.Context, ref models.SnapshotRef) (*models.Snapshot, error)
	// List returns the snapshots that match the options.
	List(ctx context.Context, options SnapshotListOptions) ([]*models.Snapshot, error)
	// Update saves the phase and error of the snapshot.
	Update(ctx context.Context, snapshot *models.Snapshot) error
	// Delete removes the snapshot. When restored microvms reference it, the files they
	// need stay until the last reference is released.
	Delete(ctx context.Context, uid string) error
	// Dir returns the directory of the snapshot on the host.
	Dir(uid string) string
	// NextGeneration increments the restore generation of the snapshot and returns
	// the new value.
	NextGeneration(ctx context.Context, uid string) (int, error)
	// AddRef records that the microvm with uid vmUID uses the files of the snapshot.
	AddRef(ctx context.Context, uid, vmUID string) error
	// ReleaseRef removes the reference of the microvm with uid vmUID. It removes the
	// files of the snapshot when the snapshot is deleted and no reference is left.
	ReleaseRef(ctx context.Context, uid, vmUID string) error
}
