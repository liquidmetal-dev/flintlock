package models

// This state represents the state of the entire Flintlock MVM.
// The state for the Firecracker MVM itself is represented in ports.MicroVMState.
type MicroVMState string

const (
	PendingState  = "pending"
	CreatedState  = "created"
	FailedState   = "failed"
	DeletingState = "deleting"
)

// MicroVM represents a microvm machine that is created via a provider.
type MicroVM struct {
	// ID is the identifier for the microvm.
	ID VMID `json:"id"`
	// Version is the version for the microvm definition.
	Version int `json:"version"`
	// Spec is the specification of the microvm.
	Spec MicroVMSpec `json:"spec"`
	// Status is the runtime status of the microvm.
	Status MicroVMStatus `json:"status"`
}

// MicroVMSpec represents the specification of a microvm machine.
type MicroVMSpec struct {
	// Provider specifies the name of the microvm provider to use.
	Provider string `json:"provider"`
	// Kernel specifies the kernel and its argments to use.
	Kernel Kernel `json:"kernel" validate:"omitempty"`
	// Initrd is an optional initial ramdisk to use.
	Initrd *Initrd `json:"initrd,omitempty"`
	// VCPU specifies how many vcpu the machine will be allocated.
	VCPU int64 `json:"vcpu" validate:"required,gte=1,lte=64"`
	// MemoryInMb is the amount of memory in megabytes that the machine will be allocated.
	MemoryInMb int64 `json:"memory_inmb" validate:"required,gte=1024,lte=32768"`
	// NetworkInterfaces specifies the network interfaces attached to the machine.
	NetworkInterfaces []NetworkInterface `json:"network_interfaces" validate:"required,dive,required"`
	// RootVolume specified the root volume to be attached to the machine.
	RootVolume Volume `json:"root_volume" validate:"required,novirtiofs"`
	// AdditionalVolumes specifies the volumes to be attached to the machine.
	AdditionalVolumes Volumes `json:"additional_volumes" validate:"onlyOneVirtioFS,multipleVolSources"`
	// Metadata allows you to specify data to be added to the metadata service. The key is the name
	// of the metadata item and the value is the base64 encoded contents of the metadata.
	Metadata map[string]string `json:"metadata"`
	// CreatedAt indicates the time the microvm was created at.
	CreatedAt int64 `json:"created_at" validate:"omitempty,datetimeInPast"`
	// UpdatedAt indicates the time the microvm was last updated.
	UpdatedAt int64 `json:"updated_at" validate:"omitempty,datetimeInPast"`
	// DeletedAt indicates the time the microvm was marked as deleted.
	DeletedAt int64 `json:"deleted_at" validate:"omitempty,datetimeInPast"`
	// PCIDevices specifies the PCI devices attached to the machine.
	PCIDevices []PCIDevice `json:"pci_devices" validate:"omitempty"`
}

// MicroVMStatus contains the runtime status of the microvm.
type MicroVMStatus struct {
	// State stores information about the last known state of the vm and the spec.
	State MicroVMState `json:"state"`
	// Volumes holds the status of the volumes.
	Volumes VolumeStatuses `json:"volumes"`
	// KernelMount holds the status of the kernel mount point.
	KernelMount *Mount `json:"kernel_mount"`
	// InitrdMount holds the status of the initrd mount point.
	InitrdMount *Mount `json:"initrd_mount"`
	// NetworkInterfaces holds the status of the network interfaces.
	NetworkInterfaces NetworkInterfaceStatuses `json:"network_interfaces"`
	// Retry is a counter about how many times we retried to reconcile.
	Retry int `json:"retry"`
	// NotBefore tells the system to do not reconcile until given timestamp.
	NotBefore int64 `json:"not_before" validate:"omitempty"`
}

type PCIDevice struct {
	// Vendor ID for the PCI Device
	VendorID string
	// Device ID for the PCI Device
	DeviceID string
}

type Initrd struct {
	// Image is the container image to use for the initrd.
	Image ContainerImage `json:"image" validate:"imageURI"`
	// Filename is the name of the initrd filename in the container.
	Filename string
}

// ContainerImage represents the address of a OCI image.
type ContainerImage string

// ListMicroVMQuery is a key-value map to query microvms.
type ListMicroVMQuery map[string]string
