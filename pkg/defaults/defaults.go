package defaults

const (
	// Domain is the reverse order domain name to use.
	Domain = "works.weave.reignited"

	// ContainerdNamespace is the name of the namespace to use with containerd.
	ContainerdNamespace = "reignite"

	// ContainerdSocket is the defaults path for the containerd socket.
	ContainerdSocket = "/run/containerd/containerd.sock"

	// ContainerdSnapshotter is the name of the default snapshotter to use for containerd.
	ContainerdSnapshotter = "devmapper"

	// FirecrackerBin is the name of the firecracker binary.
	FirecrackerBin = "firecracker"

	// ConfigurationDir is the default configuration directory.
	ConfigurationDir = "/etc/opt/reignited"

	// GRPCEndpoint is the endpoint for the gRPC server.
	GRPCAPIEndpoint = "localhost:9090"

	// HTTPAPIEndpoint is the endpoint for the HHTP proxy for the gRPC service..
	HTTPAPIEndpoint = "localhost:8090"

	// TopicMicroVMEvents is the topic name to use for microvm events.
	TopicMicroVMEvents = "microvm"
)
