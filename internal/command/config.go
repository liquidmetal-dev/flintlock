package command

import (
	"github.com/weaveworks/reignite/pkg/log"
	"github.com/weaveworks/reignite/pkg/provider"
)

// Config represents the reignited configuration.
type Config struct {
	// ConfigFilePath is the path to the shared configuration file.
	ConfigFilePath string
	// Logging contains the logging related config.
	Logging log.Config
	// MicroVM contains the microvm provider specific config.
	MicroVM provider.MicrovmProvider
	// PortNumber contains the port number for the API.
	PortNumber int
}
