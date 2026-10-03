package models

// DriveRole is what a drive presented to a microvm is used for.
type DriveRole string

const (
	// DriveRoleRoot is the drive that holds the root volume.
	DriveRoleRoot DriveRole = "root"
	// DriveRoleCloudInit is the drive that holds the cloud-init data.
	DriveRoleCloudInit DriveRole = "cloud-init"
	// DriveRoleVolume is a drive that holds an additional volume.
	DriveRoleVolume DriveRole = "volume"
)

// Drive is a block device that a provider presents to a microvm.
type Drive struct {
	// ID is the identifier of the drive given to the VMM.
	ID string
	// Role is what the drive is used for.
	Role DriveRole
	// VolumeID is the id of the volume the drive holds. It is empty for a
	// drive that holds no volume, such as the cloud-init drive.
	VolumeID string
	// GuestDevice is the name of the device inside the guest, for example vdb.
	GuestDevice string
	// ReadOnly is true when the drive is presented read-only.
	ReadOnly bool
	// Path is the location of the drive on the host. It is empty until the
	// volume has been mounted.
	Path string
}

// Drives is a list of drives in the order the guest sees them.
type Drives []Drive

// GuestDeviceForVolume returns the guest device name of the drive that holds
// an additional volume. It returns false when no drive holds the volume,
// which is the case for a volume that is not a block device.
func (d Drives) GuestDeviceForVolume(volumeID string) (string, bool) {
	for _, drive := range d {
		if drive.Role == DriveRoleVolume && drive.VolumeID == volumeID {
			return drive.GuestDevice, true
		}
	}

	return "", false
}
