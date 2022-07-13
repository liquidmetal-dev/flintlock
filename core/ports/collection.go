package ports

import (
	"time"

	"github.com/spf13/afero"
)

type Collection struct {
	Repo              MicroVMRepository
	MicrovmProviders  map[string]MicroVMService
	EventService      EventService
	IdentifierService IDService
	NetworkService    NetworkService
	ImageService      ImageService
	FileSystem        afero.Fs
	Clock             func() time.Time
}
