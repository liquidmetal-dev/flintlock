package runtime

import (
	"context"
	"fmt"
	"os"

	"github.com/liquidmetal-dev/flintlock/pkg/log"
	"github.com/liquidmetal-dev/flintlock/pkg/planner"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
)

func NewCreateDirectory(dir string, mode os.FileMode, fs afero.Fs) planner.Procedure {
	return &createDirectory{
		dir:  dir,
		mode: mode,
		fs:   fs,
	}
}

type createDirectory struct {
	dir  string
	mode os.FileMode
	fs   afero.Fs
}

// Name is the name of the procedure/operation.
func (s *createDirectory) Name() string {
	return "io_create_dir"
}

func (s *createDirectory) ShouldDo(ctx context.Context) (bool, error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step": s.Name(),
		"dir":  s.dir,
		"mode": s.mode.String(),
	})
	logger.Debug("checking if procedure should be run")
	logger.Trace("checking if directory exists")

	exists, err := s.directoryExists()
	if err != nil {
		return false, err
	}

	if !exists {
		return true, nil
	}

	logger.Trace("checking directory permissions")

	info, err := s.fs.Stat(s.dir)
	if err != nil {
		return false, fmt.Errorf("doing stat on %s: %w", s.dir, err)
	}

	expectedDirMode := s.mode | os.ModeDir
	if expectedDirMode.String() != info.Mode().String() {
		logger.Trace("permissions for directory match don't match")

		return true, nil
	}

	return false, nil
}

// Do will perform the operation/procedure.
func (s *createDirectory) Do(ctx context.Context) ([]planner.Procedure, error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step": s.Name(),
		"dir":  s.dir,
		"mode": s.mode.String(),
	})
	logger.Debug("running step to create directory")

	exists, err := s.directoryExists()
	if err != nil {
		return nil, err
	}

	if !exists {
		logger.Trace("creating directory")

		if err := s.fs.Mkdir(s.dir, s.mode); err != nil {
			return nil, fmt.Errorf("creating directory %s: %w", s.dir, err)
		}
	}

	logger.Trace("setting permissions for directory")

	if err := s.fs.Chmod(s.dir, s.mode); err != nil {
		return nil, fmt.Errorf("changing directory permissions for %s: %w", s.dir, err)
	}

	return nil, nil
}

func (s *createDirectory) directoryExists() (bool, error) {
	exists, err := afero.DirExists(s.fs, s.dir)
	if err != nil {
		return false, fmt.Errorf("checking if dir %s exists: %w", s.dir, err)
	}

	return exists, nil
}

func (s *createDirectory) Verify(ctx context.Context) error {
	return nil
}
