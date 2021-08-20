package config

import (
	"github.com/weaveworks/reignite/infrastructure/firecracker"
	"github.com/weaveworks/reignite/pkg/log"
)

// Config represents the reignited configuration.
type Config struct {
	// ConfigFilePath is the path to the shared configuration file.
	ConfigFilePath string
	// Logging contains the logging related config.
	Logging log.Config
	// GRPCEndpoint is the endpoint for the gRPC server.
	GRPCAPIEndpoint string
	// HTTPAPIEndpoint is the endpoint for the HHTP proxy for the gRPC service..
	HTTPAPIEndpoint string
	// ContainerdSocketPath is the path to the containerd socket.
	ContainerdSocketPath string
	// ContainerdNamespace is the default containerd namespace to use
	ContainerdNamespace string
	// Firecracker is the configuration for the firecracker provider
	Firecracker firecracker.Config
}
