package events

// MicroVMSpecCreated is an event for when a microvm spec is created.
type MicroVMSpecCreated struct {
	// ID is the identifier of the created microvm.
	ID string
	// Namespace is the namespace of the created microvm.
	Namespace string
	// UID is the unique id of the created microvm.
	UID string
}

// MicroVMSpecUpdated is an event for when a microvm spec is updated.
type MicroVMSpecUpdated struct {
	// ID is the identifier of the updated microvm.
	ID string
	// Namespace is the namespace of the updated microvm.
	Namespace string
	// UID is the unique id of the updated microvm.
	UID string
}

// MicroVMSpecDeleted is an event for when a microvm spec is deleted.
type MicroVMSpecDeleted struct {
	// ID is the identifier of the deleted microvm.
	ID string
	// Namespace is the namespace of the deleted microvm.
	Namespace string
	// UID is the unique id of the deleted microvm.
	UID string
}

// MicroVMRestored is an event for when a microvm has been restored from a snapshot.
type MicroVMRestored struct {
	// ID is the identifier of the restored microvm.
	ID string
	// Namespace is the namespace of the restored microvm.
	Namespace string
	// UID is the unique id of the restored microvm.
	UID string
	// SnapshotUID is the unique id of the snapshot the microvm was restored from.
	SnapshotUID string
	// Generation is the restore generation of the snapshot for this restore.
	Generation int
}
