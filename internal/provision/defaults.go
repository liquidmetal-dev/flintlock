// Package provision contains the business logic used to bootstrap a host
// for running flintlock: installing Firecracker/Cloud Hypervisor/containerd/
// flintlockd, setting up their systemd services, and configuring a
// devicemapper thinpool. It has no dependency on the CLI framework used to
// expose it - that wiring lives in internal/command/provision.
package provision

const (
	// DefaultVersion is used to indicate that the latest release of a
	// component should be installed.
	DefaultVersion = "latest"

	// DefaultBranch is the branch used when fetching raw files (such as
	// systemd unit files) from GitHub repositories.
	DefaultBranch = "main"

	// InstallPath is the directory that downloaded binaries are installed to.
	InstallPath = "/usr/local/bin"

	// FirecrackerBin is the name of the firecracker binary.
	FirecrackerBin = "firecracker"
	// FirecrackerRepo is the GitHub repository firecracker releases are published to.
	FirecrackerRepo = "firecracker-microvm/firecracker"

	// CloudHypervisorBin is the name of the cloud-hypervisor binary.
	CloudHypervisorBin = "cloud-hypervisor-static"
	// CloudHypervisorRepo is the GitHub repository cloud-hypervisor releases are published to.
	CloudHypervisorRepo = "cloud-hypervisor/cloud-hypervisor"

	// ContainerdBin is the name of the containerd binary.
	ContainerdBin = "containerd"
	// ContainerdRepo is the GitHub repository containerd releases are published to.
	ContainerdRepo = "containerd/containerd"

	// FlintlockBin is the name of the flintlockd binary.
	FlintlockBin = "flintlockd"
	// FlintlockRepo is the GitHub repository flintlock releases are published to.
	FlintlockRepo = "liquidmetal-dev/flintlock"

	// FlintlockdServiceFile is the path the flintlockd systemd unit is installed to.
	FlintlockdServiceFile = "/etc/systemd/system/flintlockd.service"
	// FlintlockdConfigPath is the path the flintlockd config file is written to.
	FlintlockdConfigPath = "/etc/opt/flintlockd/config.yaml"

	// ThinpoolProfilePath is the directory LVM thinpool profiles are written to.
	ThinpoolProfilePath = "/etc/lvm/profile"
	// DefaultThinpool is the name used for a direct-lvm backed thinpool.
	DefaultThinpool = "flintlock"
	// DefaultDevThinpool is the name used for a loopback backed thinpool.
	DefaultDevThinpool = "flintlock-dev"
	// DataSparseSize is the size of the sparse file backing the devpool data device.
	DataSparseSize = "100G"
	// MetadataSparseSize is the size of the sparse file backing the devpool metadata device.
	MetadataSparseSize = "10G"

	// SectorSize is the sector size (in bytes) used when calculating the devpool thin-pool table.
	SectorSize = 512
	// DataBlockSize is the data block size (in 512-byte sectors) used when creating the devpool thin-pool.
	DataBlockSize = 128
	// LowWaterMark is the free-space threshold (in 512-byte sectors) that triggers a dm-event for the devpool thin-pool.
	LowWaterMark = 32768
)
