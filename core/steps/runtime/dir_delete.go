package runtime

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"

	"github.com/liquidmetal-dev/flintlock/pkg/log"
	"github.com/liquidmetal-dev/flintlock/pkg/planner"
)

func NewDeleteDirectory(dir string, fs afero.Fs) planner.Procedure {
	return &deleteDirectory{
		dir: dir,
		fs:  fs,
	}
}

// NewDeleteDirectoryAndEmptyParents deletes dir, then removes each ancestor
// directory strictly below stopAt that is left empty. stopAt is an ancestor of
// dir and is never removed. Non-empty ancestors stop the walk.
func NewDeleteDirectoryAndEmptyParents(dir, stopAt string, fs afero.Fs) planner.Procedure {
	if stopAt != "" {
		// Clean so that a trailing slash matches the cleaned paths path.Dir returns.
		stopAt = path.Clean(stopAt)
	}

	return &deleteDirectory{
		dir:    dir,
		stopAt: stopAt,
		fs:     fs,
	}
}

type deleteDirectory struct {
	dir    string
	stopAt string
	fs     afero.Fs
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

		if s.stopAt != "" {
			s.removeEmptyParents()
		}
	}

	return nil, nil
}

// removeEmptyParents removes ancestors of s.dir strictly below s.stopAt, from
// the nearest up. Remove only succeeds on empty directories, so the first
// non-empty (or missing) ancestor ends the walk.
func (s *deleteDirectory) removeEmptyParents() {
	parent := path.Dir(s.dir)
	for parent != s.stopAt && strings.HasPrefix(parent, s.stopAt+"/") {
		if err := s.fs.Remove(parent); err != nil {
			return
		}

		parent = path.Dir(parent)
	}
}

func (s *deleteDirectory) targetExists() (bool, error) {
	exists, err := afero.Exists(s.fs, s.dir)
	if err != nil {
		return false, fmt.Errorf("checking if dir %s exists: %w", s.dir, err)
	}

	return exists, nil
}

func (s *deleteDirectory) Verify(_ context.Context) error {
	return nil
}
