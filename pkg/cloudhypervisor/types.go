package cloudhypervisor

// VmInfo represents information about the VM.
type VmInfo struct {
	Config           VmConfig               `json:"config"`
	State            VmState                `json:"state"`
	MemoryActualSize *int64                 `json:"memory_actual_size,omitempty"`
	DeviceTree       *map[string]DeviceNode `json:"device_tree,omitempty"`
}

// VmState is type to represent the state of a VM.
type VmState string

var (
	// VmStateCreated is a state where the the VM is created.
	VmStateCreated VmState = "Created"
	// VmStateRunning is a state where the the VM is running.
	VmStateRunning VmState = "Running"
	// VmStateShutdown is a state where the the VM is shutdown.
	VmStateShutdown VmState = "Shutdown"
	// VmStatePaused is a state where the the VM is paused.
	VmStatePaused VmState = "Paused"
	// VmStateBreakPoint is a state where the the VM is stopped at a breakpoint.
	VmStateBreakPoint VmState = "BreakPoint"
)

// VmConfig is the configuration for a VM.
type VmConfig struct {
	Cpus     *CpusConfig     `json:"cpus,omitempty"`
	Memory   *MemoryConfig   `json:"memory,omitempty"`
	Payload  PayloadConfig   `json:"payload"`
	Disks    []DiskConfig    `json:"disks,omitempty"`
	Net      []NetConfig     `json:"net,omitempty"`
	Rng      *RngConfig      `json:"rng,omitempty"`
	Balloon  *BalloonConfig  `json:"balloon,omitempty"`
	Fs       []FsConfig      `json:"fs,omitempty"`
	Pmem     []PmemConfig    `json:"pmem,omitempty"`
	Serial   *ConsoleConfig  `json:"serial,omitempty"`
	Console  *ConsoleConfig  `json:"console,omitempty"`
	Devices  []DeviceConfig  `json:"devices,omitempty"`
	Vdpa     []VdpaConfig    `json:"vdpa,omitempty"`
	Vsock    *VsockConfig    `json:"vsock,omitempty"`
	SgxEpc   []SgxEpcConfig  `json:"sgx_epc,omitempty"`
	Tdx      *TdxConfig      `json:"tdx,omitempty"`
	Numa     []NumaConfig    `json:"numa,omitempty"`
	Iommu    *bool           `json:"iommu,omitempty"`
	Watchdog *bool           `json:"watchdog,omitempty"`
	Platform *PlatformConfig `json:"platform,omitempty"`
}

// BalloonConfig holds the configuration for the balloon device.
type BalloonConfig struct {
	Size              int64 `json:"size"`
	DeflateOnOom      *bool `json:"deflate_on_oom,omitempty"`
	FreePageReporting *bool `json:"free_page_reporting,omitempty"`
}

// ConsoleMode is type to represent the mode of the console device.
type ConsoleMode string

var (
	ConsoleModeOff  ConsoleMode = "Off"
	ConsoleModePty  ConsoleMode = "Pty"
	ConsoleModeTty  ConsoleMode = "Tty"
	ConsoleModeFile ConsoleMode = "File"
	ConsoleModeNull ConsoleMode = "Null"
)

// ConsoleConfig represents the configuration for the console.
type ConsoleConfig struct {
	File  *string     `json:"file,omitempty"`
	Mode  ConsoleMode `json:"mode"`
	Iommu *bool       `json:"iommu,omitempty"`
}

// CpuAffinity is used to specify CPU affinity.
type CpuAffinity struct {
	Vcpu     *int32  `json:"vcpu,omitempty"`
	HostCpus []int32 `json:"host_cpus,omitempty"`
}

// CpuFeatures is used to enable / disable CPU features.
type CpuFeatures struct {
	Amx *bool `json:"amx,omitempty"`
}

// CpuTopology is configuration for the SPU topology.
type CpuTopology struct {
	ThreadsPerCore *int32 `json:"threads_per_core,omitempty"`
	CoresPerDie    *int32 `json:"cores_per_die,omitempty"`
	DiesPerPackage *int32 `json:"dies_per_package,omitempty"`
	Packages       *int32 `json:"packages,omitempty"`
}

// CpusConfig represents the configuration for CPUs attached to a VM.
type CpusConfig struct {
	BootVcpus   int32         `json:"boot_vcpus"`
	MaxVcpus    int32         `json:"max_vcpus"`
	KvmHyperv   *bool         `json:"kvm_hyperv,omitempty"`
	Topology    *CpuTopology  `json:"topology,omitempty"`
	MaxPhysBits *int32        `json:"max_phys_bits,omitempty"`
	Affinity    []CpuAffinity `json:"affinity,omitempty"`
	Features    *CpuFeatures  `json:"features,omitempty"`
}

// DeviceConfig represents configuration for a device attached to a VM.
type DeviceConfig struct {
	Path       string  `json:"path"`
	Iommu      *bool   `json:"iommu,omitempty"`
	PciSegment *int32  `json:"pci_segment,omitempty"`
	Id         *string `json:"id,omitempty"`
}

// DeviceNode represents a device attached to a VM.
type DeviceNode struct {
	Id        *string                  `json:"id,omitempty"`
	Resources []map[string]interface{} `json:"resources,omitempty"`
	Children  []string                 `json:"children,omitempty"`
	PciBdf    *string                  `json:"pci_bdf,omitempty"`
}

// DiskConfig represents the configuration for a disk attached to a VM.
type DiskConfig struct {
	Path              string             `json:"path"`
	Readonly          *bool              `json:"readonly,omitempty"`
	Direct            *bool              `json:"direct,omitempty"`
	Iommu             *bool              `json:"iommu,omitempty"`
	NumQueues         *int32             `json:"num_queues,omitempty"`
	QueueSize         *int32             `json:"queue_size,omitempty"`
	VhostUser         *bool              `json:"vhost_user,omitempty"`
	VhostSocket       *string            `json:"vhost_socket,omitempty"`
	RateLimiterConfig *RateLimiterConfig `json:"rate_limiter_config,omitempty"`
	PciSegment        *int32             `json:"pci_segment,omitempty"`
	Id                *string            `json:"id,omitempty"`
}

// FsConfig represents the configuration for a virtio-fs device.
type FsConfig struct {
	Tag        string  `json:"tag"`
	Socket     string  `json:"socket"`
	NumQueues  int32   `json:"num_queues"`
	QueueSize  int32   `json:"queue_size"`
	PciSegment *int32  `json:"pci_segment,omitempty"`
	Id         *string `json:"id,omitempty"`
}

// MemoryConfig represents the memory configuration for a VM.
type MemoryConfig struct {
	Size           int64              `json:"size"`
	HotplugSize    *int64             `json:"hotplug_size,omitempty"`
	HotpluggedSize *int64             `json:"hotplugged_size,omitempty"`
	Mergeable      *bool              `json:"mergeable,omitempty"`
	HotplugMethod  *string            `json:"hotplug_method,omitempty"`
	Shared         *bool              `json:"shared,omitempty"`
	Hugepages      *bool              `json:"hugepages,omitempty"`
	HugepageSize   *int64             `json:"hugepage_size,omitempty"`
	Prefault       *bool              `json:"prefault,omitempty"`
	Zones          []MemoryZoneConfig `json:"zones,omitempty"`
}

// MemoryZoneConfig represents the NUMA memory zone configuration.
type MemoryZoneConfig struct {
	Id             string  `json:"id"`
	Size           int64   `json:"size"`
	File           *string `json:"file,omitempty"`
	Mergeable      *bool   `json:"mergeable,omitempty"`
	Shared         *bool   `json:"shared,omitempty"`
	Hugepages      *bool   `json:"hugepages,omitempty"`
	HugepageSize   *int64  `json:"hugepage_size,omitempty"`
	HostNumaNode   *int32  `json:"host_numa_node,omitempty"`
	HotplugSize    *int64  `json:"hotplug_size,omitempty"`
	HotpluggedSize *int64  `json:"hotplugged_size,omitempty"`
	Prefault       *bool   `json:"prefault,omitempty"`
}

// NetConfig is the configuration for a network interface.
type NetConfig struct {
	Tap               *string            `json:"tap,omitempty"`
	Ip                *string            `json:"ip,omitempty"`
	Mask              *string            `json:"mask,omitempty"`
	Mac               *string            `json:"mac,omitempty"`
	Iommu             *bool              `json:"iommu,omitempty"`
	NumQueues         *int32             `json:"num_queues,omitempty"`
	QueueSize         *int32             `json:"queue_size,omitempty"`
	VhostUser         *bool              `json:"vhost_user,omitempty"`
	VhostSocket       *string            `json:"vhost_socket,omitempty"`
	VhostMode         *string            `json:"vhost_mode,omitempty"`
	Id                *string            `json:"id,omitempty"`
	PciSegment        *int32             `json:"pci_segment,omitempty"`
	RateLimiterConfig *RateLimiterConfig `json:"rate_limiter_config,omitempty"`
}

// NumaConfig is the NUMA configuration for a VM.
type NumaConfig struct {
	GuestNumaId    int32          `json:"guest_numa_id"`
	Cpus           []int32        `json:"cpus,omitempty"`
	Distances      []NumaDistance `json:"distances,omitempty"`
	MemoryZones    []string       `json:"memory_zones,omitempty"`
	SgxEpcSections []string       `json:"sgx_epc_sections,omitempty"`
}

// NumaDistance represents the NUMA distance.
type NumaDistance struct {
	Destination int32 `json:"destination"`
	Distance    int32 `json:"distance"`
}

// PciDeviceInfo represents information about a PCI device.
type PciDeviceInfo struct {
	Id  string `json:"id"`
	Bdf string `json:"bdf"`
}

// PayloadConfig is the configuration to boot the guest.
type PayloadConfig struct {
	Kernel    string `json:"kernel"`
	CmdLine   string `json:"cmdline,omitempty"`
	InitRamFs string `json:"initramfs,omitempty"`
}

// PlatformConfig contains information about the platform.
type PlatformConfig struct {
	NumPciSegments *int32   `json:"num_pci_segments,omitempty"`
	IommuSegments  []int32  `json:"iommu_segments,omitempty"`
	SerialNumber   string   `json:"serial_number,omitempty"`
	UUID           string   `json:"uuid,omitempty"`
	OEMStrings     []string `json:"oem_strings,omitempty"`
}

// PmemConfig represents the configuration for a PMEM device.
type PmemConfig struct {
	File          string  `json:"file"`
	Size          *int64  `json:"size,omitempty"`
	Iommu         *bool   `json:"iommu,omitempty"`
	DiscardWrites *bool   `json:"discard_writes,omitempty"`
	PciSegment    *int32  `json:"pci_segment,omitempty"`
	Id            *string `json:"id,omitempty"`
}

// RateLimiterConfig defines an IO rate limiter with independent bytes/s and ops/s limits. Limits are defined by configuring each of the _bandwidth_ and _ops_ token buckets.
type RateLimiterConfig struct {
	Bandwidth *TokenBucket `json:"bandwidth,omitempty"`
	Ops       *TokenBucket `json:"ops,omitempty"`
}

// ReceiveMigrationData is the configuration for receiving a VM migration.
type ReceiveMigrationData struct {
	ReceiverUrl string `json:"receiver_url"`
}

// RestoreConfig is the configuration for restoring a VM snapshot.
type RestoreConfig struct {
	SourceUrl string `json:"source_url"`
	Prefault  *bool  `json:"prefault,omitempty"`
}

// RngConfig is the configuration for the random number device.
type RngConfig struct {
	Src   string `json:"src"`
	Iommu *bool  `json:"iommu,omitempty"`
}

// SendMigrationData is the configuration for migrating a VM to another host.
type SendMigrationData struct {
	DestinationUrl string `json:"destination_url"`
	Local          *bool  `json:"local,omitempty"`
}

// SgxEpcConfig is the SGX configuration.
type SgxEpcConfig struct {
	Id       string `json:"id"`
	Size     int64  `json:"size"`
	Prefault *bool  `json:"prefault,omitempty"`
}

// TdxConfig is the TDX configuration.
type TdxConfig struct {
	Firmware string `json:"firmware"`
}

// TokenBucket defines a token bucket with a maximum capacity (_size_), an initial burst size (_one_time_burst_)
// and an interval for refilling purposes (_refill_time_). The refill-rate is derived from _size_ and _refill_time_,
// and it is the constant rate at which the tokens replenish. The refill process only starts happening after the
// initial burst budget is consumed. Consumption from the token bucket is unbounded in speed which allows for bursts
// bound in size by the amount of tokens available. Once the token bucket is empty, consumption speed is bound by the refill-rate.
type TokenBucket struct {
	Size         int64  `json:"size"`
	OneTimeBurst *int64 `json:"one_time_burst,omitempty"`
	RefillTime   int64  `json:"refill_time"`
}

// VdpaConfig represents the details of a vDPA device.
type VdpaConfig struct {
	Path       string  `json:"path"`
	NumQueues  int32   `json:"num_queues"`
	Iommu      *bool   `json:"iommu,omitempty"`
	PciSegment *int32  `json:"pci_segment,omitempty"`
	Id         *string `json:"id,omitempty"`
}

// VmAddDevice represents the configuration for adding a new device to a VM.
type VmAddDevice struct {
	Path  *string `json:"path,omitempty"`
	Iommu *bool   `json:"iommu,omitempty"`
	Id    *string `json:"id,omitempty"`
}

// VmRemoveDevice represents the configuration for removing a device from a VM.
type VmRemoveDevice struct {
	Id *string `json:"id,omitempty"`
}

// VmResizeZone is the target size for a NUMA memory zone.
type VmResizeZone struct {
	Id         *string `json:"id,omitempty"`
	DesiredRam *int64  `json:"desired_ram,omitempty"`
}

// VmResize is the target size for the VM.
type VmResize struct {
	DesiredVcpus   *int32 `json:"desired_vcpus,omitempty"`
	DesiredRam     *int64 `json:"desired_ram,omitempty"`
	DesiredBalloon *int64 `json:"desired_balloon,omitempty"`
}

// VmSnapshotConfig is the configuration for taking a VM snapshot.
type VmSnapshotConfig struct {
	DestinationUrl *string `json:"destination_url,omitempty"`
}

// VmmPingResponse is the details of the VMM
type VmmPingResponse struct {
	Version string `json:"version"`
}

// VsockConfig represents the configuration for a vSock device.
type VsockConfig struct {
	Cid        int64   `json:"cid"`
	Socket     string  `json:"socket"`
	Iommu      *bool   `json:"iommu,omitempty"`
	PciSegment *int32  `json:"pci_segment,omitempty"`
	Id         *string `json:"id,omitempty"`
}

// VMCoreDumpData is the configuration for a core dump.
type VMCoreDumpData struct {
	DestinationURL string `json:"destination_url"`
}

// VmCounters is the perf counters exposed from the VM.
type VmCounters map[string]map[string]int64
