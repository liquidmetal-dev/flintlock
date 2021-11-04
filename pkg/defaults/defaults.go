package defaults

import (
	"time"
)

const (
	// Domain is the reverse order domain name to use.
	Domain = "works.weave.flintlockd"

	// ContainerdNamespace is the name of the namespace to use with containerd.
	ContainerdNamespace = "flintlock"

	// ContainerdSocket is the defaults path for the containerd socket.
	ContainerdSocket = "/run/containerd/containerd.sock"

	// ContainerdVolumeSnapshotter is the name of the default snapshotter to use for volumes.
	ContainerdVolumeSnapshotter = "devmapper"

	// ContainerdKernelSnapshotter is the name of the default snapshotter to use for kernek/initrd.
	ContainerdKernelSnapshotter = "native"

	// FirecrackerBin is the name of the firecracker binary.
	FirecrackerBin = "firecracker"

	// FirecrackerDetach is the default for the flag to indicates with the child firecracker
	// processes should be run detached.
	FirecrackerDetach = true

	// FirecrackerUseAPI is the default that indicates the Firecracker microvm should be configured
	// using the API instead of using a config file.
	FirecrackerUseAPI = true

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

	// DataDirPerm is the permissions to use for data folders.
	DataDirPerm = 0o755

	// DataFilePerm is the permissions to use for data files.
	DataFilePerm = 0o644

	// MaximumRetry is the default value how many times we retry failed reconciliation.
	MaximumRetry = 10

	// ConfigFile is the name of the configuration file.
	ConfigFile = "config.yaml"
)
