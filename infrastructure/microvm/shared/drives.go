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
// A root volume in VolumeModeOverlay is two drives: the read-only base, then
// the writable disk. Neither is flagged as the root device. The guest
// contract parameters tell the init which device is which.
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

	if vm.Spec.RootVolume.EffectiveMode() == models.VolumeModeOverlay {
		base, rw := overlayDrives(vm, &vm.Spec.RootVolume)
		add(base)
		add(rw)
	} else {
		add(volumeDrive(vm, &vm.Spec.RootVolume, models.DriveRoleRoot))
	}

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

// overlayDrives returns the two drives of a root volume in overlay mode. The
// base is always read-only. The writable disk is never read-only, because
// the init writes the upper and work directories of the overlay to it. The
// paths are empty until the volume status records the mounts.
func overlayDrives(vm *models.MicroVM, vol *models.Volume) (models.Drive, models.Drive) {
	base := models.Drive{
		ID:       vol.ID + OverlayBaseDriveSuffix,
		Role:     models.DriveRoleRootBase,
		VolumeID: vol.ID,
		ReadOnly: true,
	}
	rw := models.Drive{
		ID:       vol.ID + OverlayRWDriveSuffix,
		Role:     models.DriveRoleRootRW,
		VolumeID: vol.ID,
	}

	if status, ok := vm.Status.Volumes[vol.ID]; ok && status != nil {
		rw.Path = status.Mount.Source

		if status.BaseMount != nil {
			base.Path = status.BaseMount.Source
		}
	}

	return base, rw
}

// IsDriveMounted reports whether the volume status of a microvm records the
// mount a drive needs. The cloud-init drive holds no volume and is always
// mounted. The root-base drive needs the base mount. Every other drive needs
// a status entry for its volume.
func IsDriveMounted(vm *models.MicroVM, drive models.Drive) bool {
	if drive.Role == models.DriveRoleCloudInit {
		return true
	}

	status, ok := vm.Status.Volumes[drive.VolumeID]
	if !ok {
		return false
	}

	if drive.Role == models.DriveRoleRootBase {
		return status != nil && status.BaseMount != nil
	}

	return true
}
