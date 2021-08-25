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
)

// ErrTopicNotFound is an error created when a topic with a specific name isn't found.
type ErrTopicNotFound struct {
	Name string
}

// Error returns the error message.
func (e ErrTopicNotFound) Error() string {
	return fmt.Sprintf("topic %s not found", e.Name)
}

type ErrIncorrectVMIDFormat struct {
	ActualID string
}

// Error returns the error message.
func (e ErrIncorrectVMIDFormat) Error() string {
	return fmt.Sprintf("unexpected vmid format: %s", e.ActualID)
}

func NewErrUnsupportedInterface(ifaceType string) ErrUnsupportedInterface {
	return ErrUnsupportedInterface{
		ifaceType: ifaceType,
	}
}

type ErrUnsupportedInterface struct {
	ifaceType string
}

// Error returns the error message.
func (e ErrUnsupportedInterface) Error() string {
	return fmt.Sprintf("network interface type %s is unsupported", e.ifaceType)
}

func NewVolumeNotMounted(volumeID string) ErrVolumeNotMounted {
	return ErrVolumeNotMounted{
		id: volumeID,
	}
}

// ErrVolumeNotMounted is an error used when a volume hasn't been mounted.
type ErrVolumeNotMounted struct {
	id string
}

// Error returns the error message.
func (e ErrVolumeNotMounted) Error() string {
	return fmt.Sprintf("volume %s is not mounted", e.id)
}

func NewNetworkInterfaceStatusMissing(guestIface string) ErrNetworkInterfaceStatusMissing {
	return ErrNetworkInterfaceStatusMissing{
		guestIface: guestIface,
	}
}

// NetworkInterfaceStatusMissing is an error used when a network interfaces
// status cannot be found.
type ErrNetworkInterfaceStatusMissing struct {
	guestIface string
}

// Error returns the error message.
func (e ErrNetworkInterfaceStatusMissing) Error() string {
	return fmt.Sprintf("status for network interface %s is not found", e.guestIface)
}

func NewSpecNotFound(name, namespace string) error {
	return errSpecNotFound{
		name:      name,
		namespace: namespace,
	}
}

type errSpecNotFound struct {
	name      string
	namespace string
}

// Error returns the error message.
func (e errSpecNotFound) Error() string {
	return fmt.Sprintf("microvm spec %s/%s not found", e.namespace, e.name)
}

// IsSpecNotFound tests an error to see if its a spec not found error.
func IsSpecNotFound(err error) bool {
	e := &errSpecNotFound{}

	return errors.As(err, e)
}
