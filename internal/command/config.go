package command

import (
	"github.com/weaveworks/reignite/pkg/log"
	"github.com/weaveworks/reignite/pkg/microvm"
)

// Config represents the reignited configuration.
type Config struct {
	// ConfigFilePath is the path to the shared configuration file.
	ConfigFilePath string
	// Logging contains the logging related config.
	Logging log.Config
	// MicroVM contains the microvm provider specific config.
	MicroVM microvm.Provider
	// GRPCEndpoint is the endpoint for the gRPC server.
	GRPCAPIEndpoint string
	// HTTPAPIEndpoint is the endpoint for the HHTP proxy for the gRPC service..
	HTTPAPIEndpoint string
}
