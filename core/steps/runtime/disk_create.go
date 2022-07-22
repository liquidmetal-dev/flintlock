package runtime

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/weaveworks-liquidmetal/flintlock/core/ports"
	"github.com/weaveworks-liquidmetal/flintlock/pkg/log"
	"github.com/weaveworks-liquidmetal/flintlock/pkg/planner"
)

type DiskCreateStepInput struct {
	Path           string
	VolumeName     string
	Size           string
	DiskType       ports.DiskType
	Content        []ports.DiskFile
	AlwaysRecreate bool
}

func NewDiskCreateStep(input *DiskCreateStepInput, diskSvc ports.DiskService, fs afero.Fs) planner.Procedure {
	return &diskCreateStep{
		input:   input,
		diskSvc: diskSvc,
		fs:      fs,
	}
}

type diskCreateStep struct {
	input   *DiskCreateStepInput
	diskSvc ports.DiskService
	fs      afero.Fs
}

// Name is the name of the procedure/operation.
func (s *diskCreateStep) Name() string {
	return "runtime_disk_create"
}

func (s *diskCreateStep) ShouldDo(ctx context.Context) (bool, error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step": s.Name(),
		"path": s.input.Path,
	})
	logger.Debug("checking if procedure should be run")

	if s.input.AlwaysRecreate {
		return true, nil
	}

	exists, err := afero.Exists(s.fs, s.input.Path)
	if err != nil {
		return false, fmt.Errorf("checking if disk image exsists: %w", err)
	}

	return !exists, nil
}

// Do will perform the operation/procedure.
func (s *diskCreateStep) Do(ctx context.Context) ([]planner.Procedure, error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step": s.Name(),
		"path": s.input.Path,
	})
	logger.Debug("running step to create disk")

	svcInput := ports.DiskCreateInput{
		Path:       s.input.Path,
		Size:       s.input.Size,
		VolumeName: s.input.VolumeName,
		Type:       s.input.DiskType,
		Files:      s.input.Content,
	}

	if err := s.diskSvc.Create(ctx, svcInput); err != nil {
		return nil, fmt.Errorf("creating disk: %w", err)
	}

	return nil, nil

}

func (s *diskCreateStep) Verify(ctx context.Context) error {
	return nil
}
