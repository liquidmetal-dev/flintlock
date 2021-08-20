package events

// MicroVMSpecCreated is an event for when a microvm spec is created.
type MicroVMSpecCreated struct {
	// ID is the identifier of the created microvm.
	ID string
	// Namespace is the namespace of the created microvm.
	Namespace string
}

// MicroVMSpecUpdated is an event for when a microvm spec is updated.
type MicroVMSpecUpdated struct {
	// ID is the identifier of the updated microvm.
	ID string
	// Namespace is the namespace of the updated microvm.
	Namespace string
}

// MicroVMSpecDeleted is an event for when a microvm spec is deleted.
type MicroVMSpecDeleted struct {
	// ID is the identifier of the deleted microvm.
	ID string
	// Namespace is the namespace of the deleted microvm.
	Namespace string
}
