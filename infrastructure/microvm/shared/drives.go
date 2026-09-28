package shared

import (
	"github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
)

const (
	// CloudInitDriveID is the id of the drive that holds the cloud-init image.
	CloudInitDriveID = "cloud-init"

	guestDevicePrefix = "vd"
	lettersInAlphabet = 26
)

// DriveOptions holds what differs between providers when the drives of a
// microvm are worked out.
type DriveOptions struct {
	// CloudInitImage is the location on the host of the cloud-init image.
	// When it is set, the image is presented read-only as the drive after the
	// root volume.
	CloudInitImage string
}

// BuildDrives returns the drives of a microvm in the order the guest sees
// them: the root volume, the cloud-init image if there is one, then the
// additional volumes in the order of the spec. A virtiofs volume is not a
// block device and gets no drive.
//
// The path of a drive is empty until its volume has been mounted.
func BuildDrives(vm *models.MicroVM, opts DriveOptions) (models.Drives, error) {
	if vm == nil {
		return nil, errors.ErrSpecRequired
	}

	drives := models.Drives{}
	add := func(drive models.Drive) {
		drive.GuestDevice = GuestDeviceName(len(drives))
		drives = append(drives, drive)
	}

	add(volumeDrive(vm, &vm.Spec.RootVolume, models.DriveRoleRoot))

	if opts.CloudInitImage != "" {
		add(models.Drive{
			ID:       CloudInitDriveID,
			Role:     models.DriveRoleCloudInit,
			ReadOnly: true,
			Path:     opts.CloudInitImage,
		})
	}

	for i := range vm.Spec.AdditionalVolumes {
		vol := &vm.Spec.AdditionalVolumes[i]
		if vol.Source.VirtioFS != nil {
			continue
		}

		add(volumeDrive(vm, vol, models.DriveRoleVolume))
	}

	return drives, nil
}

func volumeDrive(vm *models.MicroVM, vol *models.Volume, role models.DriveRole) models.Drive {
	drive := models.Drive{
		ID:       vol.ID,
		Role:     role,
		VolumeID: vol.ID,
		ReadOnly: vol.IsReadOnly,
	}

	if status, ok := vm.Status.Volumes[vol.ID]; ok && status != nil {
		drive.Path = status.Mount.Source
	}

	return drive
}

// GuestDeviceName returns the name that the guest gives to the virtio block
// device at the index, counting from zero: vda to vdz, then vdaa, vdab and so
// on. It returns an empty string for a negative index.
func GuestDeviceName(index int) string {
	if index < 0 {
		return ""
	}

	suffix := ""
	for i := index; i >= 0; i = i/lettersInAlphabet - 1 {
		suffix = string(rune('a'+i%lettersInAlphabet)) + suffix
	}

	return guestDevicePrefix + suffix
}
