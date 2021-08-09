package defaults

const (
	// Domain is the reverse order domain name to use.
	Domain = "works.weave.reignited"

	// ContainerdNamespace is the name of the namespace to use with containerd.
	ContainerdNamespace = "reignite"

	// FirecrackerBin is the name of the firecracker binary.
	FirecrackerBin = "firecracker"

	// ConfigurationDir is the default configuration directory.
	ConfigurationDir = "/etc/opt/reignited"

	// GRPCEndpoint is the endpoint for the gRPC server.
	GRPCAPIEndpoint = "localhost:9090"

	// HTTPAPIEndpoint is the endpoint for the HHTP proxy for the gRPC service..
	HTTPAPIEndpoint = "localhost:8090"
)
