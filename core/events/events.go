package events

// MicroVMCreated is an event for when a microvm is created.
type MicroVMCreated struct {
	// ID is the identifier of the created microvm.
	ID string
	// Namespace is the namespace of the created microvm.
	Namespace string
}

// MicroVMUpdated is an event for when a microvm is updated.
type MicroVMUpdated struct {
	// ID is the identifier of the updated microvm.
	ID string
	// Namespace is the namespace of the updated microvm.
	Namespace string
}

// MicroVMDeleted is an event for when a microvm is deleted.
type MicroVMDeleted struct {
	// ID is the identifier of the deleted microvm.
	ID string
	// Namespace is the namespace of the deleted microvm.
	Namespace string
}
