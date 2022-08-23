package cloudhypervisor

import (
	"context"
	"net"
	"net/http"

	"github.com/carlmjohnson/requests"
)

const (
	DefaultServerEndpoint = "http://localhost/api/v1/"

	PathVmmPing     = "vmm.ping"
	PathVmmShutdown = "vmm.shutdown"

	PathVmInfo           = "vm.info"
	PathVmCreate         = "vm.create"
	PathVmDelete         = "vm.delete"
	PathVmBoot           = "vm.boot"
	PathVmPause          = "vm.pause"
	PathVmResume         = "vm.resume"
	PathVmShutdown       = "vm.shutdown"
	PathVmReboot         = "vm.reboot"
	PathVmPowerButton    = "vm.power-button"
	PathVmResize         = "vm.resize"
	PathVmResizeZone     = "vm.resize-zone"
	PathVmAddDevice      = "vm.add-device"
	PathVmRemoveDevice   = "vm.remove-device"
	PathVmAddDisk        = "vm.add-disk"
	PathVmAddFs          = "vm.add-fs"
	PathVmAddPmem        = "vm.add-pmem"
	PathAddNetworkDevice = "vm.add-net"
	PathAddVsockDevice   = "vm.add-vsock"
	PathAddVdpaDevice    = "vm.add-vdpa"
	PathCoreDumpCreate   = "vm.coredump"
	PathSnapshotCreate   = "vm.snapshot"
	PathSnapshotRestore  = "vm.restore"
	PathMigrationReceive = "vm.receive-migration"
	PathMigrationSend    = "vm.send-migration"
)

// Client represents a client for the Cloud Hypervisor API.
type Client interface {
	// VmmPing checks for API server availability.
	VmmPing(ctx context.Context) (*VmmPingResponse, error)
	// VmmShutdown shuts down the cloud-hypervisor vmm.
	VmmShutdown(ctx context.Context) error
	// Info returns general information about the cloud-hypervisor Virtual Machine (VM) instance.
	Info(ctx context.Context) (*VmInfo, error)
	// Create will create the cloud-hypervisor Virtual Machine (VM) instance. The instance is not booted, only created.
	Create(ctx context.Context, config *VmConfig) error
	// Delete will delete the cloud-hypervisor Virtual Machine (VM) instance.
	Delete(ctx context.Context) error
	// Boot will boot the previously created VM instance.
	Boot(ctx context.Context) error
	// Pause a previously booted VM instance.
	Pause(ctx context.Context) error
	// Resume a previously paused VM instance.
	Resume(ctx context.Context) error
	// Shutdown the VM instance.
	Shutdown(ctx context.Context) error
	// Reboot the VM instance.
	Reboot(ctx context.Context) error
	// PowerButton simulates pressing the equivalent of a physical power button.
	PowerButton(ctx context.Context) error
	// Resize will change the vpcus/ram/balloon (a.k.a resize).
	Resize(ctx context.Context, config *VmResize) error
	// ResizeZone will resize a memory zone.
	ResizeZone(ctx context.Context, config *VmResizeZone) error
	// AddDevice is used to add a new device to the VM.
	AddDevice(ctx context.Context, config *VmAddDevice) (*PciDeviceInfo, error)
	// RemoveDevice is used to remove a device from the VM.
	RemoveDevice(ctx context.Context, config *VmRemoveDevice) error
	// AddDisk will add a new disk to the VM.
	AddDisk(ctx context.Context, config *DiskConfig) (*PciDeviceInfo, error)
	// AddFs will add a new virtio-fs device to the VM.
	AddFs(ctx context.Context, config *FsConfig) (*PciDeviceInfo, error)
	// AddPmemDevice will add a new pmem device to the VM.
	AddPmemDevice(ctx context.Context, config *PmemConfig) (*PciDeviceInfo, error)
	// AddNetworkDevice will add a new network device to the VM.
	AddNetworkDevice(ctx context.Context, config *NetConfig) (*PciDeviceInfo, error)
	// AddVsockDevice will add a new vsock device to the VM.
	AddVsockDevice(ctx context.Context, config *VsockConfig) (*PciDeviceInfo, error)
	// AddVdpaDevice will add a new vdpa device to the VM.
	AddVdpaDevice(ctx context.Context, config *VdpaConfig) (*PciDeviceInfo, error)
	// Snapshot will create a snapshot of the VM.
	Snapshot(ctx context.Context, config *VmSnapshotConfig) error
	// CoreDump will take a core dump of the VM.
	CoreDump(ctx context.Context, config *VMCoreDumpData) error
	// Restore will restore a VM from a snapshot.
	Restore(ctx context.Context, config *RestoreConfig) error
	// ReceiveMigration will receive a VM migration from a URL.
	ReceiveMigration(ctx context.Context, config *ReceiveMigrationData) error
	// SendMigration will send a VM migration to a URL.
	SendMigration(ctx context.Context, config *SendMigrationData) error
}

type client struct {
	builder *requests.Builder
}

// New will create a new cloud hypervisor client.
func New(socketPath string) Client {
	t := &http.Transport{
		DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", socketPath)
		},
	}

	return &client{
		builder: requests.URL(DefaultServerEndpoint).Transport(t),
	}
}

// VmmPing checks for API server availability.
func (c *client) VmmPing(ctx context.Context) (*VmmPingResponse, error) {
	resp := &VmmPingResponse{}

	if err := c.builder.Clone().Path(PathVmmPing).ToJSON(resp).Fetch(ctx); err != nil {
		return nil, err
	}

	return resp, nil
}

// VmmShutdown shuts down the cloud-hypervisor vmm.
func (c *client) VmmShutdown(ctx context.Context) error {
	return c.builder.Clone().Path(PathVmmShutdown).Put().Fetch(ctx)
}

// Info returns general information about the cloud-hypervisor Virtual Machine (VM) instance.
func (c *client) Info(ctx context.Context) (*VmInfo, error) {
	data := &VmInfo{}

	if err := c.builder.Clone().Path(PathVmInfo).
		ToJSON(data).
		Fetch(ctx); err != nil {
		return nil, err
	}
	return data, nil
}

// Create will create the cloud-hypervisor Virtual Machine (VM) instance. The instance is not booted, only created
func (c *client) Create(ctx context.Context, config *VmConfig) error {
	return c.builder.Clone().Path(PathVmCreate).Put().BodyJSON(config).Fetch(ctx)
}

// Delete will delete the cloud-hypervisor Virtual Machine (VM) instance.
func (c *client) Delete(ctx context.Context) error {
	return c.builder.Clone().Path(PathVmDelete).Put().Fetch(ctx)
}

// Boot will boot the previously created VM instance.
func (c *client) Boot(ctx context.Context) error {
	return c.
		builder.
		Clone().
		Path(PathVmBoot).
		AddValidator(CustomErrValidator(map[int]string{
			404: "The VM instance could not boot because it is not created yet",
		})).
		Put().
		Fetch(ctx)
}

// Pause a previously booted VM instance.
func (c *client) Pause(ctx context.Context) error {
	return c.
		builder.
		Clone().
		Path(PathVmPause).
		AddValidator(CustomErrValidator(map[int]string{
			404: "The VM instance could not pause because it is not created yet",
			405: "The VM instance could not pause because it is not booted",
		})).
		Put().
		Fetch(ctx)
}

// Resume a previously paused VM instance.
func (c *client) Resume(ctx context.Context) error {
	return c.
		builder.
		Clone().
		Path(PathVmResume).
		AddValidator(CustomErrValidator(map[int]string{
			404: "The VM instance could not resume because it is not booted yet",
			405: "The VM instance could not resume because it is not paused",
		})).
		Put().
		Fetch(ctx)
}

// Shutdown will shut down the VM instance down.
func (c *client) Shutdown(ctx context.Context) error {
	return c.
		builder.
		Clone().
		Path(PathVmShutdown).
		AddValidator(CustomErrValidator(map[int]string{
			404: "The VM instance could not shut down because is not created",
			405: "The VM instance could not shut down because it is not started",
		})).
		Put().
		Fetch(ctx)
}

// Reboot the VM instance.
func (c *client) Reboot(ctx context.Context) error {
	return c.
		builder.
		Clone().
		Path(PathVmReboot).
		AddValidator(CustomErrValidator(map[int]string{
			404: "The VM instance could not reboot because it is not created",
			405: "The VM instance could not reboot because it is not booted",
		})).
		Put().
		Fetch(ctx)
}

// PowerButton simulates pressing the equivalent of a physical power button.
func (c *client) PowerButton(ctx context.Context) error {
	return c.
		builder.
		Clone().
		Path(PathVmPowerButton).
		AddValidator(CustomErrValidator(map[int]string{
			404: "The button could not be triggered because it is not created yet",
			405: "The button could not be triggered because it is not booted",
		})).
		Put().
		Fetch(ctx)
}

// Resize will change the vpcus/ram/balloon (a.k.a resize).
func (c *client) Resize(ctx context.Context, config *VmResize) error {
	return c.
		builder.
		Clone().
		Path(PathVmResize).
		AddValidator(CustomErrValidator(map[int]string{
			404: "The VM instance could not be resized because it is not created",
		})).
		Put().
		Fetch(ctx)
}

// ResizeZone will resize a memory zone.
func (c *client) ResizeZone(ctx context.Context, config *VmResizeZone) error {
	return c.
		builder.
		Clone().
		Path(PathVmResizeZone).
		AddValidator(CustomErrValidator(map[int]string{
			500: "The memory zone could not be resized",
		})).
		Put().
		Fetch(ctx)
}

// AddDevice is used to add a new device to the the VM.
func (c *client) AddDevice(ctx context.Context, config *VmAddDevice) (*PciDeviceInfo, error) {
	data := &PciDeviceInfo{}
	if err := c.
		builder.
		Clone().
		Path(PathVmAddDevice).
		AddValidator(CustomErrValidator(map[int]string{
			404: "The new device could not be added to the VM instance",
		})).
		Put().Handle(ToJSONForCode(200, data)).
		Fetch(ctx); err != nil {
		return nil, err
	}

	return data, nil
}

// RemoveDevice is used to remove a device from the VM.
func (c *client) RemoveDevice(ctx context.Context, config *VmRemoveDevice) error {
	return c.
		builder.
		Clone().
		Path(PathVmRemoveDevice).
		AddValidator(CustomErrValidator(map[int]string{
			404: "The device could not be removed from the VM instance",
		})).
		Put().
		Fetch(ctx)
}

// AddDisk will add a new disk to the VM.
func (c *client) AddDisk(ctx context.Context, config *DiskConfig) (*PciDeviceInfo, error) {
	data := &PciDeviceInfo{}
	if err := c.
		builder.
		Clone().
		Path(PathVmAddDisk).
		AddValidator(CustomErrValidator(map[int]string{
			500: "The new disk could not be added to the VM instance",
		})).
		Put().Handle(ToJSONForCode(200, data)).
		Fetch(ctx); err != nil {
		return nil, err
	}

	return data, nil
}

// AddFs will add a new virtio-fs device to the VM.
func (c *client) AddFs(ctx context.Context, config *FsConfig) (*PciDeviceInfo, error) {
	data := &PciDeviceInfo{}
	if err := c.
		builder.
		Clone().
		Path(PathVmAddFs).
		AddValidator(CustomErrValidator(map[int]string{
			500: "The new virtio-fs device could not be added to the VM instance",
		})).
		Put().Handle(ToJSONForCode(200, data)).
		Fetch(ctx); err != nil {
		return nil, err
	}

	return data, nil
}

// AddPmemDevice will add a new pmem device to the VM.
func (c *client) AddPmemDevice(ctx context.Context, config *PmemConfig) (*PciDeviceInfo, error) {
	data := &PciDeviceInfo{}
	if err := c.
		builder.
		Clone().
		Path(PathVmAddPmem).
		AddValidator(CustomErrValidator(map[int]string{
			500: "The new pmem device could not be added to the VM instance",
		})).
		Put().Handle(ToJSONForCode(200, data)).
		Fetch(ctx); err != nil {
		return nil, err
	}

	return data, nil
}

// AddNetworkDevice will add a new network device to the VM.
func (c *client) AddNetworkDevice(ctx context.Context, config *NetConfig) (*PciDeviceInfo, error) {
	data := &PciDeviceInfo{}
	if err := c.
		builder.
		Clone().
		Path(PathAddNetworkDevice).
		AddValidator(CustomErrValidator(map[int]string{
			500: "The new network device could not be added to the VM instance",
		})).
		Put().Handle(ToJSONForCode(200, data)).
		Fetch(ctx); err != nil {
		return nil, err
	}

	return data, nil
}

// AddVsockDevice will add a new vsock device to the VM.
func (c *client) AddVsockDevice(ctx context.Context, config *VsockConfig) (*PciDeviceInfo, error) {
	data := &PciDeviceInfo{}
	if err := c.
		builder.
		Clone().
		Path(PathAddVsockDevice).
		AddValidator(CustomErrValidator(map[int]string{
			500: "The new vSock device could not be added to the VM instance",
		})).
		Put().Handle(ToJSONForCode(200, data)).
		Fetch(ctx); err != nil {
		return nil, err
	}

	return data, nil
}

// AddVdpaDevice will add a new vdpa device to the VM.
func (c *client) AddVdpaDevice(ctx context.Context, config *VdpaConfig) (*PciDeviceInfo, error) {
	data := &PciDeviceInfo{}
	if err := c.
		builder.
		Clone().
		Path(PathAddVdpaDevice).
		AddValidator(CustomErrValidator(map[int]string{
			500: "The new vDPA device could not be added to the VM instance",
		})).
		Put().Handle(ToJSONForCode(200, data)).
		Fetch(ctx); err != nil {
		return nil, err
	}

	return data, nil
}

// Snapshot will create a snapshot of the VM.
func (c *client) Snapshot(ctx context.Context, config *VmSnapshotConfig) error {
	return c.
		builder.
		Clone().
		Path(PathSnapshotCreate).
		AddValidator(CustomErrValidator(map[int]string{
			404: "The VM instance could not be snapshotted because it is not created",
			405: "The VM instance could not be snapshotted because it is not booted",
		})).
		Put().
		Fetch(ctx)
}

// CoreDump will take a core dump of the VM.
func (c *client) CoreDump(ctx context.Context, config *VMCoreDumpData) error {
	return c.
		builder.
		Clone().
		Path(PathCoreDumpCreate).
		AddValidator(CustomErrValidator(map[int]string{
			404: "The VM instance could not be coredumped because it is not created",
			405: "The VM instance could not be coredumped because it is not booted",
		})).
		Put().
		Fetch(ctx)
}

// Restore will restore a VM from a snapshot.
func (c *client) Restore(ctx context.Context, config *RestoreConfig) error {
	return c.
		builder.
		Clone().
		Path(PathSnapshotRestore).
		AddValidator(CustomErrValidator(map[int]string{
			404: "The VM instance could not be restored because it is already created",
		})).
		Put().
		Fetch(ctx)
}

// ReceiveMigration will receive a VM migration from a URL.
func (c *client) ReceiveMigration(ctx context.Context, config *ReceiveMigrationData) error {
	return c.
		builder.
		Clone().
		Path(PathMigrationReceive).
		AddValidator(CustomErrValidator(map[int]string{
			500: "The VM migration could not be received",
		})).
		Put().
		Fetch(ctx)
}

// SendMigration will send a VM migration to a URL.
func (c *client) SendMigration(ctx context.Context, config *SendMigrationData) error {
	return c.
		builder.
		Clone().
		Path(PathMigrationSend).
		AddValidator(CustomErrValidator(map[int]string{
			500: "The VM migration could not be sent",
		})).
		Put().
		Fetch(ctx)
}
