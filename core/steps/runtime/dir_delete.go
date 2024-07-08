package runtime

import (
	"context"
	"fmt"

	"github.com/liquidmetal-dev/flintlock/pkg/log"
	"github.com/liquidmetal-dev/flintlock/pkg/planner"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
)

func NewDeleteDirectory(dir string, fs afero.Fs) planner.Procedure {
	return &deleteDirectory{
		dir: dir,
		fs:  fs,
	}
}

type deleteDirectory struct {
	dir string
	fs  afero.Fs
}

// Name is the name of the procedure/operation.
func (s *deleteDirectory) Name() string {
	return "io_delete_dir"
}

func (s *deleteDirectory) ShouldDo(ctx context.Context) (bool, error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step": s.Name(),
		"dir":  s.dir,
	})
	logger.Debug("checking if procedure should be run")

	logger.Trace("checking if directory exists")

	return s.targetExists()
}

// Do will perform the operation/procedure.
func (s *deleteDirectory) Do(ctx context.Context) ([]planner.Procedure, error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step": s.Name(),
		"dir":  s.dir,
	})
	logger.Debugf("running step to delete directory %s", s.dir)

	exists, err := s.targetExists()
	if err != nil {
		return nil, err
	}

	if exists {
		logger.Trace("deleting directory")

		if err := s.fs.RemoveAll(s.dir); err != nil {
			return nil, fmt.Errorf("deleting directory %s: %w", s.dir, err)
		}
	}

	return nil, nil
}

func (s *deleteDirectory) targetExists() (bool, error) {
	exists, err := afero.Exists(s.fs, s.dir)
	if err != nil {
		return false, fmt.Errorf("checking if dir %s exists: %w", s.dir, err)
	}

	return exists, nil
}

func (s *deleteDirectory) Verify(ctx context.Context) error {
	return nil
}
