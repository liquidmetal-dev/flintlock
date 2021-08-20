package models

// Mount represents a volume mount point.
type Mount struct {
	// Type specifies the type of the mount (e.g. device or directory).
	Type MountType
	// Source is the location of the mounted volume.
	Source string
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
