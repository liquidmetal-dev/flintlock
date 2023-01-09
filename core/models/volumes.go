package models

// Volume represents a volume to be attached to a microvm machine.
type Volume struct {
	// ID is the uinique identifier of the volume.
	ID string `json:"id"`
	// IsReadOnly specifies that the volume is to be mounted readonly.
	IsReadOnly bool `json:"is_read_only,omitempty"`
	// Source is where the volume will be sourced from.
	Source VolumeSource `json:"source"`
	// PartitionID is the uuid of the boot partition.
	PartitionID string `json:"partition_id,omitempty"`
	// Size is the size to resize this volume to.
	Size int32 `json:"size,omitempty"`
	// MountPoint allows you to optionally specify a mount point for the volume. This only
	// applied to additional volumes and it will use cloud-init to mount the volumes.
	MountPoint string `json:"mount_point,omitempty"`
}

// Volumes represents a collection of volumes.
type Volumes []Volume

// GetByID will get a volume status by id.
func (v Volumes) GetByID(id string) *Volume {
	for _, vol := range v {
		if vol.ID == id {
			return &vol
		}
	}

	return nil
}

// HasMountableVolumes returns true if any of the volumes
// have a mount point defined
func (v Volumes) HasMountableVolumes() bool {
	for _, vol := range v {
		if vol.MountPoint != "" {
			return true
		}
	}

	return false
}

// VolumeSource is the source of a volume. Based loosely on the volumes in Kubernetes Pod specs.
type VolumeSource struct {
	// Container is used to specify a source of a volume as a OCI container.
	Container *ContainerVolumeSource `json:"container,omitempty"`
}

// ContainerDriveSource represents the details of a volume coming from a OCI image.
type ContainerVolumeSource struct {
	// Image is the OCI image to use.
	Image ContainerImage `json:"image"`
}

// Mount represents a volume mount point.
type Mount struct {
	// Type specifies the type of the mount (e.g. device or directory).
	Type MountType `json:"type"`
	// Source is the location of the mounted volume.
	Source string `json:"source"`
}

// MountType is a type representing the type of mount.
type MountType string

const (
	// MountTypeDev represents a mount point that is a block device.
	MountTypeDev MountType = "dev"
	// MountTypeHostPath represents a mount point that is a directory on the host.
	MountTypeHostPath MountType = "hostpath"
)

// ImageUse is a type representing the how an image will be used.
type ImageUse string

const (
	// ImageUseVolume represents the usage of af an image for a volume.
	ImageUseVolume ImageUse = "volume"
	// ImageUseKernel represents the usage of af an image for a kernel.
	ImageUseKernel ImageUse = "kernel"
	// ImageUseKernel represents the usage of af an image for a initial ramdisk.
	ImageUseInitrd ImageUse = "initrd"
)

// VolumeStatus holds status information about the volumes.
type VolumeStatus struct {
	// Mount is the mount point information for the volume.
	Mount Mount `json:"mount"`
}

// VolumeStatuses represents a collection of volume status.
type VolumeStatuses map[string]*VolumeStatus
