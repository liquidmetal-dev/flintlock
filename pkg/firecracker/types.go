package firecracker

// InstanceState is the state of a Firecracker instance, as reported by Describe.
type InstanceState string

const (
	InstanceStateNotStarted InstanceState = "Not started"
	InstanceStateRunning    InstanceState = "Running"
	InstanceStatePaused     InstanceState = "Paused"
)

// InstanceInfo is general information about a Firecracker instance.
type InstanceInfo struct {
	// AppName is the name of the application.
	AppName string `json:"app_name"`
	// ID is the identifier of the microvm.
	ID string `json:"id"`
	// State is the current state of the instance.
	State InstanceState `json:"state"`
	// VMMVersion is the version of the VMM.
	VMMVersion string `json:"vmm_version"`
}

// VMState is a state a microvm can be moved to.
type VMState string

const (
	VMStatePaused  VMState = "Paused"
	VMStateResumed VMState = "Resumed"
)

// VM is the body of a request to change the state of a microvm.
type VM struct {
	State VMState `json:"state"`
}

// Fault is the body of an error response from the Firecracker API.
type Fault struct {
	FaultMessage string `json:"fault_message"`
}

// SnapshotType is the type of a snapshot.
type SnapshotType string

const (
	// SnapshotTypeFull is a snapshot of all the guest memory.
	SnapshotTypeFull SnapshotType = "Full"
)

// SnapshotCreateParams is the body of a request to create a snapshot.
type SnapshotCreateParams struct {
	// SnapshotType is the type of the snapshot.
	SnapshotType SnapshotType `json:"snapshot_type"`
	// SnapshotPath is the path of the file for the microvm state.
	SnapshotPath string `json:"snapshot_path"`
	// MemFilePath is the path of the file for the guest memory.
	MemFilePath string `json:"mem_file_path"`
}

// MemoryBackendType is the type of the backend that gives the guest memory at
// snapshot load.
type MemoryBackendType string

const (
	// MemoryBackendTypeFile maps the guest memory from a file.
	MemoryBackendTypeFile MemoryBackendType = "File"
)

// MemoryBackend is the backend that gives the guest memory at snapshot load.
type MemoryBackend struct {
	// BackendType is the type of the backend.
	BackendType MemoryBackendType `json:"backend_type"`
	// BackendPath is the path of the memory file.
	BackendPath string `json:"backend_path"`
}

// NetworkOverride replaces the host device of a network interface at snapshot
// load.
type NetworkOverride struct {
	// IfaceID is the identifier of the network interface in the snapshot.
	IfaceID string `json:"iface_id"`
	// HostDevName is the name of the new host device.
	HostDevName string `json:"host_dev_name"`
}

// SnapshotLoadParams is the body of a request to load a snapshot.
type SnapshotLoadParams struct {
	// SnapshotPath is the path of the file with the microvm state.
	SnapshotPath string `json:"snapshot_path"`
	// MemBackend is the backend that gives the guest memory.
	MemBackend MemoryBackend `json:"mem_backend"`
	// NetworkOverrides replace the host devices of network interfaces.
	NetworkOverrides []NetworkOverride `json:"network_overrides,omitempty"`
	// ResumeVM resumes the microvm after the load when true.
	ResumeVM bool `json:"resume_vm"`
}
