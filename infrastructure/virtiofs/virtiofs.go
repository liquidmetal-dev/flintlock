package virtiofs

import (
	"context"
	"github.com/spf13/afero"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/internal/config"
)

// New will create a new instance of the VirtioFS.
func New(cfg *config.Config,
	fs afero.Fs,
) (ports.VirtioFSService) {
	return &vFSService{
		config:          cfg,
		fs:              fs,
	}
}

type vFSService struct {
	config *config.Config
	fs     afero.Fs
}

// Create will create a new disk.
func (s *vFSService) Create(_ context.Context, input ports.VirtioFSCreateInput) error {
	return nil
}