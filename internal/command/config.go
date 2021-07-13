package command

import (
	"github.com/weaveworks/reignite/pkg/log"
	"github.com/weaveworks/reignite/pkg/microvm/firecracker"
	"github.com/weaveworks/reignite/pkg/state"
)

// Config represents the reignited configuration.
type Config struct {
	// ConfigFilePath is the path to the shared configuration file.
	ConfigFilePath string
	// Logging contains the logging related config.
	Logging log.Config
	// State contains the state related config.
	State state.Config
	// MicroVM contains the microvm provider specific config.
	MicroVM MicroVM
	// PortNumber contains the port number for the API.
	PortNumber int
}

type MicroVM struct {
	// Firecracker contains the firecracker config.
	Firecracker *firecracker.Config
}

func (c *Config) Validate() error {
	//TODO: add validation
	return nil
}
