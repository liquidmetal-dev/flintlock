package plans

import (
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/spf13/afero"
)

// Providers input is a type to be used as input to plans.
type ProvidersInput struct {
	MicroVMService ports.MicroVMService
	MicroVMRepo    ports.MicroVMRepository
	EventService   ports.EventService
	ImageService   ports.ImageService
	NetworkService ports.NetworkService

	FS afero.Fs
}
