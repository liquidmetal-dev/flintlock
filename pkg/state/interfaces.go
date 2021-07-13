package state

// StateProvider is the interface for a state provider
type StateProvider interface {
	// Get will get the state for a specific microvm.
	Get(vmid string) State
}

type State interface {
	Exists() bool
	Ensure() error

	Root() string
	VolumesRoot() string
	KernelRoot() string
	PIDFile() string
	LogsRoot() string
	SocksFile() string
}
