package ports

import (
	"time"

	"github.com/spf13/afero"
)

type Collection struct {
	Repo              MicroVMRepository
	Provider          MicroVMService
	EventService      EventService
	IdentifierService IDService
	NetworkService    NetworkService
	ImageService      ImageService
	DiskService       DiskService
	FileSystem        afero.Fs
	Clock             func() time.Time
}
