package models

import "time"

const (
	// SnapshotDescriptorFormatVersion is the format version of the
	// compatibility descriptor that this flintlock writes.
	SnapshotDescriptorFormatVersion = 1
	// SnapshotDescriptorFilename is the name of the compatibility descriptor
	// in the snapshot directory.
	SnapshotDescriptorFilename = "descriptor.json"
)

const (
	// VMMTypeFirecracker is the VMM type of a Firecracker microvm. It is the
	// same as the name of the Firecracker provider.
	VMMTypeFirecracker = "firecracker"
	// VMMTypeCloudHypervisor is the VMM type of a Cloud Hypervisor microvm.
	// It is the same as the name of the Cloud Hypervisor provider.
	VMMTypeCloudHypervisor = "cloudhypervisor"
)

// SnapshotDescriptor is the compatibility descriptor of a snapshot. It holds
// the facts that decide if a host can restore the snapshot, and flintlock can
// read it without the guest memory, the VMM state or the volume layers.
//
// The descriptor is plaintext. It has no field for the metadata map of the
// source microvm, because that map often holds secrets.
type SnapshotDescriptor struct {
	// FormatVersion is the format version of the descriptor.
	FormatVersion int `json:"format_version"`
	// VMMType is the type of the VMM that took the snapshot.
	VMMType string `json:"vmm_type"`
	// VMMVersion is the exact version of the VMM that took the snapshot.
	VMMVersion string `json:"vmm_version"`
	// FCSnapshotVersion is the snapshot format version of Firecracker. It is
	// empty for other VMMs.
	FCSnapshotVersion string `json:"fc_snapshot_version,omitempty"`
	// FlintlockVersion is the version of flintlock that took the snapshot.
	FlintlockVersion string `json:"flintlock_version"`
	// Arch is the CPU architecture of the source host, as GOARCH names it.
	Arch string `json:"arch"`
	// CPUVendor is the CPU vendor of the source host.
	CPUVendor string `json:"cpu_vendor"`
	// CPUModel is the CPU model name of the source host.
	CPUModel string `json:"cpu_model"`
	// CPUMicrocode is the CPU microcode version of the source host.
	CPUMicrocode string `json:"cpu_microcode"`
	// CPUTemplate is the CPU template of the source microvm, if any.
	CPUTemplate string `json:"cpu_template,omitempty"`
	// HostKernel is the kernel release of the source host.
	HostKernel string `json:"host_kernel"`
	// VCPUs is the vCPU count of the source microvm.
	VCPUs int64 `json:"vcpus"`
	// MemoryMB is the memory size of the source microvm in megabytes.
	MemoryMB int64 `json:"memory_mb"`
	// KernelImage is the image the kernel of the source microvm came from.
	KernelImage SnapshotImage `json:"kernel_image"`
	// InitrdImage is the image the initrd of the source microvm came from,
	// if it had one.
	InitrdImage *SnapshotImage `json:"initrd_image,omitempty"`
	// Volumes are the mounted volumes of the source microvm.
	Volumes []SnapshotVolume `json:"volumes"`
	// CreatedAt is the time the snapshot was taken.
	CreatedAt time.Time `json:"created_at"`
	// Source identifies the source microvm.
	Source SnapshotSource `json:"source"`
	// Labels are the labels the client attached to the snapshot.
	Labels map[string]string `json:"labels,omitempty"`
}

// SnapshotImage is an image that a snapshot depends on.
type SnapshotImage struct {
	// Reference is the image reference.
	Reference string `json:"reference"`
	// Digest is the digest of the image.
	Digest string `json:"digest"`
}

// SnapshotVolume is a mounted volume of the source microvm.
type SnapshotVolume struct {
	// DriveID is the identifier of the drive.
	DriveID string `json:"drive_id"`
	// Order is the position of the drive in the order the guest sees.
	Order int `json:"order"`
	// SizeBytes is the size of the volume in bytes.
	SizeBytes int64 `json:"size_bytes"`
	// BlockSize is the block size of the volume in bytes.
	BlockSize int64 `json:"block_size"`
	// Filesystem is the filesystem type of the volume.
	Filesystem string `json:"filesystem"`
	// BaseDigest is the digest of the base block image of the volume.
	BaseDigest string `json:"base_digest,omitempty"`
}

// SnapshotSource identifies the source microvm of a snapshot.
type SnapshotSource struct {
	// UID is the unique identifier of the source microvm.
	UID string `json:"uid"`
	// Name is the name of the source microvm.
	Name string `json:"name"`
	// Namespace is the namespace of the source microvm.
	Namespace string `json:"namespace"`
}

// HostFacts are the facts of a host that decide if it can restore a snapshot.
type HostFacts struct {
	// VMMType is the type of the VMM.
	VMMType string `json:"vmm_type"`
	// VMMVersion is the exact version of the VMM binary.
	VMMVersion string `json:"vmm_version"`
	// FCSnapshotVersion is the snapshot format version of Firecracker. It is
	// empty for other VMMs.
	FCSnapshotVersion string `json:"fc_snapshot_version,omitempty"`
	// Arch is the CPU architecture, as GOARCH names it.
	Arch string `json:"arch"`
	// CPUVendor is the vendor_id of the first processor.
	CPUVendor string `json:"cpu_vendor"`
	// CPUModel is the model name of the first processor.
	CPUModel string `json:"cpu_model"`
	// CPUMicrocode is the microcode version of the first processor.
	CPUMicrocode string `json:"cpu_microcode"`
	// HostKernel is the kernel release, as uname -r prints it.
	HostKernel string `json:"host_kernel"`
}
