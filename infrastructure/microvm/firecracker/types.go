package firecracker

// VmmConfig contains the configuration of the microvm.
// Based on the rust structure from firecracker:
// https://github.com/firecracker-microvm/firecracker/blob/0690010524001b606f67c1a65c67f3c27883183f/src/vmm/src/resources.rs#L51.
type VmmConfig struct {
	// Balloon hols the balloon device configuration.
	Balloon *BalloonDeviceConfig `json:"balloon,omitempty"`
	// BlockDevices is the configuration for the drives.
	BlockDevices []BlockDeviceConfig `json:"drives"`
	// BootSourec is the boot source configuration for the microvm.
	BootSource BootSourceConfig `json:"boot-source"`
	// Logger is the logger configuration.
	Logger *LoggerConfig `json:"logger,omitempty"`
	// MachineConfig contains the microvm machine config.
	MachineConfig MachineConfig `json:"machine-config"`
	// Metrics is the metrics configuration.
	Metrics *MetricsConfig `json:"metrics,omitempty"`
	// Mmds is the configuration for the metadata service
	Mmds *MMDSConfig `json:"mmds-config,omitempty"`
	// NetDevices is the configuration for the microvm network devices.
	NetDevices []NetworkInterfaceConfig `json:"network-interfaces"`
	// VsockDevice is the configuration for the vsock device.
	VsockDevice *VsockDeviceConfig `json:"vsock,omitempty"`
}

type MachineConfig struct {
	// VcpuCount is the number of vcpu to start.
	VcpuCount int64 `json:"vcpu_count"`
	// MemSizeMib is the memory size in MiB.
	MemSizeMib int64 `json:"mem_size_mib"`
	// SMT enables or disabled hyperthreading.
	SMT bool `json:"smt"`
	// CPUTemplate is a CPU template that it is used to filter the CPU features exposed to the guest.
	CPUTemplate *string `json:"cpu_template,omitempty"`
	// TrackDirtyPages enables or disables dirty page tracking. Enabling allows incremental snapshots.
	TrackDirtyPages bool `json:"track_dirty_pages"`
}

type CacheType string

const (
	// CacheTypeUnsafe indovates the flushing mechanic will be advertised to
	// the guest driver, but the operation will be a noop.
	CacheTypeUnsafe CacheType = "Unsafe"
	// CacheTypeWriteBack indicates the flushing mechanic will be advertised
	// to the guest driver and flush requests coming from the guest will be
	// performed using `fsync`.
	CacheTypeWriteBack CacheType = "WriteBack"
)

type FileEngineType string

const (
	// FileEngineTypeSync specifies using a synchronous engine based on blocking system calls.
	FileEngineTypeSync = FileEngineType("Sync")
	// FileEngineTypeAsync specifies using a asynchronous engine based on io_uring.
	FileEngineTypeAsync = FileEngineType("Async")
)

// BlockDeviceConfig contains the configuration for a microvm block device.
type BlockDeviceConfig struct {
	// ID is the unique identifier of the drive.
	ID string `json:"drive_id"`
	// PathOnHost is the path of the drive on the host machine.
	PathOnHost string `json:"path_on_host"`
	// IsRootDevice when true makes the current device the root block device.
	// Setting this flag to true will mount the block device in the
	// guest under /dev/vda unless the partuuid is present.
	IsRootDevice bool `json:"is_root_device"`
	// PartUUID represents the unique id of the boot partition of this device. It is
	// optional and it will be used only if the `IsRootDevice` field is true.
	PartUUID string `json:"partuuid,omitempty"`
	// IsReadOnly when true the drive is opened in read-only mode. Otherwise, the
	// drive is opened as read-write.
	IsReadOnly bool `json:"is_read_only"`
	// CacheType indicates whether the drive will ignore flush requests coming from
	// the guest driver.
	CacheType CacheType `json:"cache_type"`
	// RateLimiter is the config for rate limiting the I/O operations.
	// RateLimiter *RateLimiterConfig `json:"rate_limiter"`
}

// BootSourceConfig holds the configuration for the boot source of a microvm.
type BootSourceConfig struct {
	// KernelImagePage is the path of the kernel image.
	KernelImagePage string `json:"kernel_image_path"`
	// InitrdPath is the path of the initrd, if there is one.
	InitrdPath *string `json:"initrd_path,omitempty"`
	// BootArgs contains the boot arguments to pass to the kernel. If this field is uninitialized, the default
	// kernel command line is used: `reboot=k panic=1 pci=off nomodules 8250.nr_uarts=0`.
	BootArgs *string `json:"boot_args,omitempty"`
}

// NetworkInterfaceConfig is the configuration for a network interface of a microvm.
type NetworkInterfaceConfig struct {
	// IfaceID is the ID of the guest network interface.
	IfaceID string `json:"iface_id"`
	// HostDevName is the host level path for the guest network interface.
	HostDevName string `json:"host_dev_name"`
	// GuestMAC is the mac address to use.
	GuestMAC string `json:"guest_mac,omitempty"`
	// RxRateLimiter is the rate limiter for received packages.
	// RxRateLimiter *RateLimiterConfig `json:"rx_rate_limiter,omitempty"`
	// TxRateLimiter is the rate limiter for transmitted packages.
	// TxRateLimiter *RateLimiterConfig `json:"tx_rate_limiter,omitempty"`
}

type LogLevel string

const (
	LogLevelError   LogLevel = "Error"
	LogLevelWarning LogLevel = "Warning"
	LogLevelInfo    LogLevel = "Info"
	LogLevelDebug   LogLevel = "Debug"
)

// LoggerConfig holds the configuration for the logger.
type LoggerConfig struct {
	// LogPath is the named pipe or file used as output for logs.
	LogPath string `json:"log_path"`
	// Level is the level of the logger.
	Level LogLevel `json:"level"`
	// ShowLevel when set to true the logger will append to the output the severity of the log entry.
	ShowLevel bool `json:"show_level"`
	// ShowLogOrigin when set to true the logger will append the origin of the log entry.
	ShowLogOrigin bool `json:"show_log_origin"`
}

// BalloonDeviceConfig holds the configuration for the memory balloon device.
type BalloonDeviceConfig struct {
	// AmountMib is the target balloon size in MiB.
	AmountMib int64 `json:"amount_mib"`
	// DeflateOnOOM  when set to true will deflate the balloon in case the guest is out of memory.
	DeflateOnOOM bool `json:"deflate_on_oom"`
	// StatsPollingInterval is the interval in seconds between refreshing statistics.
	StatsPollingInterval int64 `json:"stats_polling_interval_s"`
}

// MetrcsConfig contains the configuration for the microvm metrics.
type MetricsConfig struct {
	// Path is the named pipe or file used as output for metrics.
	Path string `json:"metrics_path"`
}

type MMDSVersion string

const (
	MMDSVersion1 = MMDSVersion("V1")
	MMDSVersion2 = MMDSVersion("V2")
)

// MMDSConfig is the config related to the mmds.
type MMDSConfig struct {
	// Version specifies the MMDS version to use. If not specified it will default to V1. Supported values are V1 & V2.
	Version MMDSVersion `json:"version,omitempty"`
	// NetworkInterfaces specifies the interfaces that allow forwarding packets to MMDS.
	NetworkInterfaces []string `json:"network_interfaces,omitempty"`
	// IPV4Address is the MMDS IPv4 configured address.
	IPV4Address *string `json:"ipv4_address,omitempty"`
}

type VsockDeviceConfig struct {
	// ID of the vsock device.
	ID string `json:"vsock_id"`
	// GuestCID is a 32-bit Context Identifier (CID) used to identify the guest.
	GuestCID int64 `json:"guest_cid"`
	// UDSPath is the path to local unix socket.
	UDSPath string `json:"uds_path"`
}

// Metadata represents metadata in the MMDS.
type Metadata struct {
	Latest map[string]string `json:"latest"`
}

// InstanceState is a type that represents the running state of a Firecracker instance.
type InstanceState string

const (
	// InstanceStateNotStarted the instance hasn't started running yet.
	InstanceStateNotStarted InstanceState = "Not started"
	// InstanceStateRunning the instance is running.
	InstanceStateRunning InstanceState = "Running"
	// InstanceStatePaused the instance is currently paused.
	InstanceStatePaused InstanceState = "Paused"
)
