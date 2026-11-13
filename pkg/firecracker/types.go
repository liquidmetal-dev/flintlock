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
