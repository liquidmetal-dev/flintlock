package cloudhypervisor

// VmInfo Virtual Machine information
type VmInfo struct {
	Config           VmConfig               `json:"config"`
	State            VmState                `json:"state"`
	MemoryActualSize *int64                 `json:"memory_actual_size,omitempty"`
	DeviceTree       *map[string]DeviceNode `json:"device_tree,omitempty"`
}

type VmState string

var (
	VmStateCreated    VmState = "Created"
	VmStateRunning    VmState = "Running"
	VmStateShutdown   VmState = "Shutdown"
	VmStatePaused     VmState = "Paused"
	VmStateBreakPoint VmState = "BreakPoint"
)

// VmConfig Virtual machine configuration
type VmConfig struct {
	Cpus      *CpusConfig      `json:"cpus,omitempty"`
	Memory    *MemoryConfig    `json:"memory,omitempty"`
	Kernel    KernelConfig     `json:"kernel"`
	Initramfs *InitramfsConfig `json:"initramfs,omitempty"`
	Cmdline   *CmdLineConfig   `json:"cmdline,omitempty"`
	Disks     []DiskConfig     `json:"disks,omitempty"`
	Net       []NetConfig      `json:"net,omitempty"`
	Rng       *RngConfig       `json:"rng,omitempty"`
	Balloon   *BalloonConfig   `json:"balloon,omitempty"`
	Fs        []FsConfig       `json:"fs,omitempty"`
	Pmem      []PmemConfig     `json:"pmem,omitempty"`
	Serial    *ConsoleConfig   `json:"serial,omitempty"`
	Console   *ConsoleConfig   `json:"console,omitempty"`
	Devices   []DeviceConfig   `json:"devices,omitempty"`
	Vdpa      []VdpaConfig     `json:"vdpa,omitempty"`
	Vsock     *VsockConfig     `json:"vsock,omitempty"`
	SgxEpc    []SgxEpcConfig   `json:"sgx_epc,omitempty"`
	Tdx       *TdxConfig       `json:"tdx,omitempty"`
	Numa      []NumaConfig     `json:"numa,omitempty"`
	Iommu     *bool            `json:"iommu,omitempty"`
	Watchdog  *bool            `json:"watchdog,omitempty"`
	Platform  *PlatformConfig  `json:"platform,omitempty"`
}

// BalloonConfig struct for BalloonConfig
type BalloonConfig struct {
	Size int64 `json:"size"`
	// Deflate balloon when the guest is under memory pressure.
	DeflateOnOom *bool `json:"deflate_on_oom,omitempty"`
	// Enable guest to report free pages.
	FreePageReporting *bool `json:"free_page_reporting,omitempty"`
}

// CmdLineConfig struct for CmdLineConfig
type CmdLineConfig struct {
	Args string `json:"args"`
}

// ConsoleConfig struct for ConsoleConfig
type ConsoleConfig struct {
	File  *string `json:"file,omitempty"`
	Mode  string  `json:"mode"`
	Iommu *bool   `json:"iommu,omitempty"`
}

// CpuAffinity struct for CpuAffinity
type CpuAffinity struct {
	Vcpu     *int32  `json:"vcpu,omitempty"`
	HostCpus []int32 `json:"host_cpus,omitempty"`
}

// CpuFeatures struct for CpuFeatures
type CpuFeatures struct {
	Amx *bool `json:"amx,omitempty"`
}

// CpuTopology struct for CpuTopology
type CpuTopology struct {
	ThreadsPerCore *int32 `json:"threads_per_core,omitempty"`
	CoresPerDie    *int32 `json:"cores_per_die,omitempty"`
	DiesPerPackage *int32 `json:"dies_per_package,omitempty"`
	Packages       *int32 `json:"packages,omitempty"`
}

// CpusConfig struct for CpusConfig
type CpusConfig struct {
	BootVcpus   int32         `json:"boot_vcpus"`
	MaxVcpus    int32         `json:"max_vcpus"`
	Topology    *CpuTopology  `json:"topology,omitempty"`
	MaxPhysBits *int32        `json:"max_phys_bits,omitempty"`
	Affinity    []CpuAffinity `json:"affinity,omitempty"`
	Features    *CpuFeatures  `json:"features,omitempty"`
}

// DeviceConfig struct for DeviceConfig
type DeviceConfig struct {
	Path       string  `json:"path"`
	Iommu      *bool   `json:"iommu,omitempty"`
	PciSegment *int32  `json:"pci_segment,omitempty"`
	Id         *string `json:"id,omitempty"`
}

// DeviceNode struct for DeviceNode
type DeviceNode struct {
	Id        *string                  `json:"id,omitempty"`
	Resources []map[string]interface{} `json:"resources,omitempty"`
	Children  []string                 `json:"children,omitempty"`
	PciBdf    *string                  `json:"pci_bdf,omitempty"`
}

// DiskConfig struct for DiskConfig
type DiskConfig struct {
	Path              string             `json:"path"`
	Readonly          *bool              `json:"readonly,omitempty"`
	Direct            *bool              `json:"direct,omitempty"`
	Iommu             *bool              `json:"iommu,omitempty"`
	NumQueues         *int32             `json:"num_queues,omitempty"`
	QueueSize         *int32             `json:"queue_size,omitempty"`
	VhostUser         *bool              `json:"vhost_user,omitempty"`
	VhostSocket       *string            `json:"vhost_socket,omitempty"`
	PollQueue         *bool              `json:"poll_queue,omitempty"`
	RateLimiterConfig *RateLimiterConfig `json:"rate_limiter_config,omitempty"`
	PciSegment        *int32             `json:"pci_segment,omitempty"`
	Id                *string            `json:"id,omitempty"`
}

// FsConfig struct for FsConfig
type FsConfig struct {
	Tag        string  `json:"tag"`
	Socket     string  `json:"socket"`
	NumQueues  int32   `json:"num_queues"`
	QueueSize  int32   `json:"queue_size"`
	Dax        bool    `json:"dax"`
	CacheSize  int64   `json:"cache_size"`
	PciSegment *int32  `json:"pci_segment,omitempty"`
	Id         *string `json:"id,omitempty"`
}

// InitramfsConfig struct for InitramfsConfig
type InitramfsConfig struct {
	Path string `json:"path"`
}

// KernelConfig struct for KernelConfig
type KernelConfig struct {
	Path string `json:"path"`
}

// MemoryConfig struct for MemoryConfig
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

// MemoryZoneConfig struct for MemoryZoneConfig
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

// NetConfig struct for NetConfig
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

// NumaConfig struct for NumaConfig
type NumaConfig struct {
	GuestNumaId    int32          `json:"guest_numa_id"`
	Cpus           []int32        `json:"cpus,omitempty"`
	Distances      []NumaDistance `json:"distances,omitempty"`
	MemoryZones    []string       `json:"memory_zones,omitempty"`
	SgxEpcSections []string       `json:"sgx_epc_sections,omitempty"`
}

// NumaDistance struct for NumaDistance
type NumaDistance struct {
	Destination int32 `json:"destination"`
	Distance    int32 `json:"distance"`
}

// PciDeviceInfo Information about a PCI device
type PciDeviceInfo struct {
	Id  string `json:"id"`
	Bdf string `json:"bdf"`
}

// PlatformConfig struct for PlatformConfig
type PlatformConfig struct {
	NumPciSegments *int32  `json:"num_pci_segments,omitempty"`
	IommuSegments  []int32 `json:"iommu_segments,omitempty"`
}

// PmemConfig struct for PmemConfig
type PmemConfig struct {
	File          string  `json:"file"`
	Size          *int64  `json:"size,omitempty"`
	Iommu         *bool   `json:"iommu,omitempty"`
	Mergeable     *bool   `json:"mergeable,omitempty"`
	DiscardWrites *bool   `json:"discard_writes,omitempty"`
	PciSegment    *int32  `json:"pci_segment,omitempty"`
	Id            *string `json:"id,omitempty"`
}

// RateLimiterConfig Defines an IO rate limiter with independent bytes/s and ops/s limits. Limits are defined by configuring each of the _bandwidth_ and _ops_ token buckets.
type RateLimiterConfig struct {
	Bandwidth *TokenBucket `json:"bandwidth,omitempty"`
	Ops       *TokenBucket `json:"ops,omitempty"`
}

// ReceiveMigrationData struct for ReceiveMigrationData
type ReceiveMigrationData struct {
	ReceiverUrl string `json:"receiver_url"`
}

// RestoreConfig struct for RestoreConfig
type RestoreConfig struct {
	SourceUrl string `json:"source_url"`
	Prefault  *bool  `json:"prefault,omitempty"`
}

// RngConfig struct for RngConfig
type RngConfig struct {
	Src   string `json:"src"`
	Iommu *bool  `json:"iommu,omitempty"`
}

// SendMigrationData struct for SendMigrationData
type SendMigrationData struct {
	DestinationUrl string `json:"destination_url"`
	Local          *bool  `json:"local,omitempty"`
}

// SgxEpcConfig struct for SgxEpcConfig
type SgxEpcConfig struct {
	Id       string `json:"id"`
	Size     int64  `json:"size"`
	Prefault *bool  `json:"prefault,omitempty"`
}

// TdxConfig struct for TdxConfig
type TdxConfig struct {
	// Path to the firmware that will be used to boot the TDx guest up.
	Firmware string `json:"firmware"`
}

// TokenBucket Defines a token bucket with a maximum capacity (_size_), an initial burst size (_one_time_burst_) and an interval for refilling purposes (_refill_time_). The refill-rate is derived from _size_ and _refill_time_, and it is the constant rate at which the tokens replenish. The refill process only starts happening after the initial burst budget is consumed. Consumption from the token bucket is unbounded in speed which allows for bursts bound in size by the amount of tokens available. Once the token bucket is empty, consumption speed is bound by the refill-rate.
type TokenBucket struct {
	// The total number of tokens this bucket can hold.
	Size int64 `json:"size"`
	// The initial size of a token bucket.
	OneTimeBurst *int64 `json:"one_time_burst,omitempty"`
	// The amount of milliseconds it takes for the bucket to refill.
	RefillTime int64 `json:"refill_time"`
}

// VdpaConfig struct for VdpaConfig
type VdpaConfig struct {
	Path       string  `json:"path"`
	NumQueues  int32   `json:"num_queues"`
	Iommu      *bool   `json:"iommu,omitempty"`
	PciSegment *int32  `json:"pci_segment,omitempty"`
	Id         *string `json:"id,omitempty"`
}

// VmAddDevice struct for VmAddDevice
type VmAddDevice struct {
	Path  *string `json:"path,omitempty"`
	Iommu *bool   `json:"iommu,omitempty"`
	Id    *string `json:"id,omitempty"`
}

// VmRemoveDevice struct for VmRemoveDevice
type VmRemoveDevice struct {
	Id *string `json:"id,omitempty"`
}

// VmResizeZone struct for VmResizeZone
type VmResizeZone struct {
	Id *string `json:"id,omitempty"`
	// desired memory zone size in bytes
	DesiredRam *int64 `json:"desired_ram,omitempty"`
}

// VmResize struct for VmResize
type VmResize struct {
	DesiredVcpus *int32 `json:"desired_vcpus,omitempty"`
	// desired memory ram in bytes
	DesiredRam *int64 `json:"desired_ram,omitempty"`
	// desired balloon size in bytes
	DesiredBalloon *int64 `json:"desired_balloon,omitempty"`
}

// VmSnapshotConfig struct for VmSnapshotConfig
type VmSnapshotConfig struct {
	DestinationUrl *string `json:"destination_url,omitempty"`
}

// VmmPingResponse Virtual Machine Monitor information
type VmmPingResponse struct {
	Version string `json:"version"`
}

// VsockConfig struct for VsockConfig
type VsockConfig struct {
	// Guest Vsock CID
	Cid int64 `json:"cid"`
	// Path to UNIX domain socket, used to proxy vsock connections.
	Socket     string  `json:"socket"`
	Iommu      *bool   `json:"iommu,omitempty"`
	PciSegment *int32  `json:"pci_segment,omitempty"`
	Id         *string `json:"id,omitempty"`
}
