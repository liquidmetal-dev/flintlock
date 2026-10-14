package shared

import (
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/internal/config"
	"github.com/liquidmetal-dev/flintlock/internal/overlayinit"
)

const (
	// OverlayBaseDriveSuffix is appended to the root volume id to name the
	// drive that holds the read-only base.
	OverlayBaseDriveSuffix = "_base"
	// OverlayRWDriveSuffix is appended to the root volume id to name the
	// drive that holds the writable disk.
	OverlayRWDriveSuffix = "_rw"

	// OverlayBasePath is where the read-only base of an overlay root volume
	// is, relative to the state directory of the microvm.
	OverlayBasePath = "volumes/root.base"
	// OverlayRWPath is where the writable disk of an overlay root volume is,
	// relative to the state directory of the microvm.
	OverlayRWPath = "volumes/root.rw"

	guestDeviceDir = "/dev/"
)

// SetOverlayKernelParams sets the guest contract parameters of an overlay
// root volume on a kernel command line: the contract version and the guest
// devices of the base and of the writable disk, taken from the drives. It
// replaces a value the spec set for one of them, so the init sees each key
// once. It does nothing when the drives hold no overlay root.
func SetOverlayKernelParams(cmdLine config.KernelCmdLine, drives models.Drives) {
	base, hasBase := drives.ByRole(models.DriveRoleRootBase)
	rw, hasRW := drives.ByRole(models.DriveRoleRootRW)

	if !hasBase || !hasRW {
		return
	}

	cmdLine.Set(overlayinit.KeyContract, overlayinit.ContractVersion)
	cmdLine.Set(overlayinit.KeyBase, guestDeviceDir+base.GuestDevice)
	cmdLine.Set(overlayinit.KeyRW, guestDeviceDir+rw.GuestDevice)
}
