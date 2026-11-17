package events

// SnapshotCreated is an event for when a snapshot record is created.
type SnapshotCreated struct {
	// UID is the unique id of the created snapshot.
	UID string
	// Name is the name of the created snapshot.
	Name string
	// Namespace is the namespace of the created snapshot.
	Namespace string
	// SourceUID is the unique id of the microvm the snapshot is taken from.
	SourceUID string
}

// SnapshotDeleted is an event for when a snapshot is deleted.
type SnapshotDeleted struct {
	// UID is the unique id of the deleted snapshot.
	UID string
	// Name is the name of the deleted snapshot.
	Name string
	// Namespace is the namespace of the deleted snapshot.
	Namespace string
	// SourceUID is the unique id of the microvm the snapshot was taken from.
	SourceUID string
}

// SnapshotUpdated is an event for when the phase of a snapshot changes.
type SnapshotUpdated struct {
	// UID is the unique id of the snapshot.
	UID string
	// Name is the name of the snapshot.
	Name string
	// Namespace is the namespace of the snapshot.
	Namespace string
	// SourceUID is the unique id of the microvm the snapshot is taken from.
	SourceUID string
	// Phase is the new phase of the snapshot.
	Phase string
	// Error is the error of a failed snapshot.
	Error string
}
