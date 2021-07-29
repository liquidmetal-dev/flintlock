package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MicroVMSpec represents the specification of a microvm machine.
type MicroVMSpec struct {
	// Provider is the name of the microvm provider. Defaults to firecracker.
	Provider string `json:"provider,omitempty"`
	// Kernel specifies the kernel and its argments to use.
	Kernel Kernel `json:"kernel" validate:"required"`
	// InitrdImage is an optional initial ramdisk to use.
	InitrdImage ContainerImage `json:"initrd_image,omitempty"`
	// VCPU specifies how many vcpu the machine will be allocated.
	VCPU int64 `json:"vcpu" validate:"required,gt=0"`
	// MemoryInMb is the amount of memory in megabytes that the machine will be allocated.
	MemoryInMb int64 `json:"memory_inmb" validate:"required,gt=0"`
	// NetworkInterfaces specifies the network interfaces attached to the machine.
	NetworkInterfaces []NetworkInterface `json:"network_interfaces" validate:"required"`
	// Volumes specifies the volumes to be attached to the the machine.
	Volumes []Volume `json:"volumes" validate:"required"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MicroVM represents a microvm machine that is created via a provider.
type MicroVM struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// MachineSpec is the spec of the machine.
	Spec MicroVMSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// MicroVMList represents a list on microvms.
type MicroVMList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MicroVM `json:"items"`
}
