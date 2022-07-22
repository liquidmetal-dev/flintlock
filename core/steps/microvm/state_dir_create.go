package microvm

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"

	"github.com/weaveworks-liquidmetal/flintlock/core/errors"
	"github.com/weaveworks-liquidmetal/flintlock/core/models"
	"github.com/weaveworks-liquidmetal/flintlock/core/steps/runtime"
	"github.com/weaveworks-liquidmetal/flintlock/pkg/defaults"
	"github.com/weaveworks-liquidmetal/flintlock/pkg/log"
	"github.com/weaveworks-liquidmetal/flintlock/pkg/planner"
)

func NewStateDirStep(rootStateDir string, vm *models.MicroVM, fs afero.Fs) planner.Procedure {
	return &stateDirStep{
		vm:           vm,
		fs:           fs,
		rootStateDir: rootStateDir,
	}
}

type stateDirStep struct {
	rootStateDir string
	vm           *models.MicroVM
	fs           afero.Fs
}

// Name is the name of the procedure/operation.
func (s *stateDirStep) Name() string {
	return "microvm_state_dir_create"
}

func (s *stateDirStep) ShouldDo(ctx context.Context) (bool, error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step": s.Name(),
		"vmid": s.vm.ID,
	})
	logger.Debug("checking if procedure should be run")

	if s.vm == nil {
		return false, errors.ErrSpecRequired
	}

	if s.vm.Status.RuntimeStateDir == "" {
		return true, nil
	}

	exists, err := afero.Exists(s.fs, s.vm.Status.RuntimeStateDir)
	if err != nil {
		return false, fmt.Errorf("checking if vm state directory exists %s: %w", s.vm.Status.RuntimeStateDir, err)
	}

	return !exists, nil
}

// Do will perform the operation/procedure.
func (s *stateDirStep) Do(ctx context.Context) ([]planner.Procedure, error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step": s.Name(),
		"vmid": s.vm.ID,
	})
	logger.Debug("creating microvm runtime state directory")

	if s.vm.Status.RuntimeStateDir == "" {
		s.vm.Status.RuntimeStateDir = fmt.Sprintf("%s/vm/%s", s.rootStateDir, s.vm.ID)
	}

	return []planner.Procedure{
		runtime.NewCreateDirectory(s.vm.Status.RuntimeStateDir, defaults.DataDirPerm, s.fs),
	}, nil
}

func (s *stateDirStep) Verify(ctx context.Context) error {
	return nil
}
