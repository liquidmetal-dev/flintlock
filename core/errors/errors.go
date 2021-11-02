package errors

import (
	"errors"
	"fmt"
)

var (
	ErrSpecRequired            = errors.New("microvm spec is required")
	ErrVMIDRequired            = errors.New("id for microvm is required")
	ErrNameRequired            = errors.New("name is required")
	ErrNamespaceRequired       = errors.New("namespace is required")
	ErrKernelImageRequired     = errors.New("kernel image is required")
	ErrVolumeRequired          = errors.New("no volumes specified, at least 1 volume is required")
	ErrRootVolumeRequired      = errors.New("a root volume is required")
	ErrNoMount                 = errors.New("no image mount point")
	ErrNoVolumeMount           = errors.New("no volume mount point")
	ErrParentIfaceRequired     = errors.New("a parent network device name is required")
	ErrGuestDeviceNameRequired = errors.New("a guest device name is required")
	ErrUnsupportedIfaceType    = errors.New("unsupported network interface type")
	ErrIfaceNotFound           = errors.New("network interface not found")
	ErrMissingStatusInfo       = errors.New("status is not defined")
)

// TopicNotFoundError is an error created when a topic with a specific name isn't found.
type TopicNotFoundError struct {
	Name string
}

// Error returns the error message.
func (e TopicNotFoundError) Error() string {
	return fmt.Sprintf("topic %s not found", e.Name)
}

type IncorrectVMIDFormatError struct {
	ActualID string
}

// Error returns the error message.
func (e IncorrectVMIDFormatError) Error() string {
	return fmt.Sprintf("unexpected vmid format: %s", e.ActualID)
}

func NewErrUnsupportedInterface(ifaceType string) UnsupportedInterfaceError {
	return UnsupportedInterfaceError{
		ifaceType: ifaceType,
	}
}

type UnsupportedInterfaceError struct {
	ifaceType string
}

// Error returns the error message.
func (e UnsupportedInterfaceError) Error() string {
	return fmt.Sprintf("network interface type %s is unsupported", e.ifaceType)
}

func NewVolumeNotMounted(volumeID string) VolumeNotMountedError {
	return VolumeNotMountedError{
		id: volumeID,
	}
}

// VolumeNotMountedError is an error used when a volume hasn't been mounted.
type VolumeNotMountedError struct {
	id string
}

// Error returns the error message.
func (e VolumeNotMountedError) Error() string {
	return fmt.Sprintf("volume %s is not mounted", e.id)
}

func NewNetworkInterfaceStatusMissing(guestIface string) NetworkInterfaceStatusMissingError {
	return NetworkInterfaceStatusMissingError{
		guestIface: guestIface,
	}
}

// NetworkInterfaceStatusMissing is an error used when a network interfaces
// status cannot be found.
type NetworkInterfaceStatusMissingError struct {
	guestIface string
}

// Error returns the error message.
func (e NetworkInterfaceStatusMissingError) Error() string {
	return fmt.Sprintf("status for network interface %s is not found", e.guestIface)
}

func NewSpecNotFound(name, namespace, version string) error {
	return specNotFoundError{
		name:      name,
		namespace: namespace,
		version:   version,
	}
}

type specNotFoundError struct {
	name      string
	namespace string
	version   string
}

// Error returns the error message.
func (e specNotFoundError) Error() string {
	if e.version == "" {
		return fmt.Sprintf("microvm spec %s/%s not found", e.namespace, e.name)
	}

	return fmt.Sprintf("microvm spec %s/%s not found with version %s", e.namespace, e.name, e.version)
}

// IsSpecNotFound tests an error to see if its a spec not found error.
func IsSpecNotFound(err error) bool {
	e := &specNotFoundError{}

	return errors.As(err, e)
}
