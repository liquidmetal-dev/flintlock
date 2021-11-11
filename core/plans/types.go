package plans

import (
	"github.com/spf13/afero"

	"github.com/weaveworks/flintlock/core/ports"
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
