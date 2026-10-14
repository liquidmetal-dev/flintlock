package overlayinit

import "errors"

var (
	// ErrMissingParam is returned when a required kernel parameter is absent.
	ErrMissingParam = errors.New("missing kernel parameter")
	// ErrInvalidParam is returned when a kernel parameter has a value the init cannot use.
	ErrInvalidParam = errors.New("invalid kernel parameter")
	// ErrUnknownContract is returned when the contract version is not one the init knows.
	ErrUnknownContract = errors.New("unknown contract version")
	// ErrDeviceTimeout is returned when a block device does not appear within DeviceTimeout.
	ErrDeviceTimeout = errors.New("timed out waiting for block device")
	// ErrNotBlockDevice is returned when a device path exists but is not a block device.
	ErrNotBlockDevice = errors.New("not a block device")
)
