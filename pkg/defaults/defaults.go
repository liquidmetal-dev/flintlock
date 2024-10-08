package defaults

import (
	"time"
)

const (
	// Domain is the reverse order domain name to use.
	Domain = "dev.liquidmetal.flintlockd"

	// ContainerdNamespace is the name of the namespace to use with containerd.
	ContainerdNamespace = "flintlock"

	// ContainerdSocket is the defaults path for the containerd socket.
	ContainerdSocket = "/run/containerd/containerd.sock"

	// ContainerdVolumeSnapshotter is the name of the snapshotter used for volumes.
	ContainerdVolumeSnapshotter = "devmapper"

	// ContainerdKernelSnapshotter is the name of the default snapshotter to use for kernek/initrd.
	ContainerdKernelSnapshotter = "native"

	// FirecrackerBin is the name of the firecracker binary.
	FirecrackerBin = "firecracker"

	// FirecrackerDetach is the default for the flag to indicates with the child firecracker
	// processes should be run detached.
	FirecrackerDetach = true

	// CloudHypervisorBin is the name of the Cloud Hypervisor binary.
	CloudHypervisorBin = "cloud-hypervisor-static"

	// CloudHypervisorDetach is the default for the flag to indicates with the child cloud-hypervisor
	// processes should be run detached.
	CloudHypervisorDetach = true

	// ConfigurationDir is the default configuration directory.
	ConfigurationDir = "/etc/opt/flintlockd"

	// StateRootDir is the default directory to use for state information.
	StateRootDir = "/var/lib/flintlock"

	// GRPCEndpoint is the endpoint for the gRPC server.
	GRPCAPIEndpoint = "localhost:9090"

	// HTTPAPIEndpoint is the endpoint for the HHTP proxy for the gRPC service..
	HTTPAPIEndpoint = "localhost:8090"

	// TopicMicroVMEvents is the topic name to use for microvm events.
	TopicMicroVMEvents = "/microvm"

	// MicroVMNamespace is the default namespace to use for microvms.
	MicroVMNamespace = "default"

	// ResyncPeriod is the default resync period duration.
	ResyncPeriod time.Duration = 10 * time.Minute

	// DeleteVMTimeout is the default timeout for deleting a microvm.
	DeleteVMTimeout time.Duration = 10 * time.Second

	// DataDirPerm is the permissions to use for data folders.
	DataDirPerm = 0o755

	// DataFilePerm is the permissions to use for data files.
	DataFilePerm = 0o644

	// MaximumRetry is the default value how many times we retry failed reconciliation.
	MaximumRetry = 10

	// Namespace is the default MicroVM namespace if one is not provided by the user.
	Namespace = "default"

	// VCPU is the default number if VCPUs for a MicroVM if one is not provided by the user.
	VCPU = 2

	// MemoryInMb is the default amount of RAM for a MicroVM if one is not provided by the user.
	MemoryInMb = 1024
)
