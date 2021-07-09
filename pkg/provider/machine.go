package provider

type VirtualMachine struct {
	ID string
	VirtualMachineSpec
}

type VirtualMachineSpec struct {
	Kernel            KernelSpec         // Required
	InitrdImage       Image              // Optional
	VCPU              uint64             // Required
	MemoryInMb        uint64             // Required
	NetworkInterfaces []NetworkInterface // Min len 1
	Drives            []Drive            // Min len 1
}

type KernelSpec struct {
	Image   Image
	CmdLine string
}

type Image string

type NetworkInterface struct {
	AllowMetadataRequests bool
	GuestMAC              string
	HostDeviceName        string
	GuestIfaceID          string
	//TODO: Rate limiting
	//TODO: CNI
	CNI *CNIConfig
}

type Drive struct {
	ID           string
	IsRootDevice bool
	ReadOnly     bool
	MountPoint   string
	Source       DriveSource
	PartitionID  string
	Size         DriveSize
	//TODO: rate limiting
}

type DriveSource struct {
	Container *ContainerDriveSource
	HostPath  *HostPathDriveSource
	//CSI *CSIDriveSource
}

type ContainerDriveSource struct {
	Image Image
}

type HostPathDriveSource struct {
	Path string
	Type HostPathType
}

type HostPathType string

const (
	HostPathRawFile HostPathType = "RawFile"
)

type DriveSize uint64

type CNIConfig struct {
	CNIPath      string
	CNIConfigDir string
}
