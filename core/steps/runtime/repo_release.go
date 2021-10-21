package runtime

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/weaveworks/flintlock/core/models"
	"github.com/weaveworks/flintlock/core/ports"
	"github.com/weaveworks/flintlock/pkg/log"
	"github.com/weaveworks/flintlock/pkg/planner"
)

func NewRepoRelease(vm *models.MicroVM, repo ports.MicroVMRepository) planner.Procedure {
	return &repoRelease{
		vm:   vm,
		repo: repo,
	}
}

type repoRelease struct {
	vm   *models.MicroVM
	repo ports.MicroVMRepository
}

// Name is the name of the procedure/operation.
func (s *repoRelease) Name() string {
	return "runtime_repo_release"
}

func (s *repoRelease) ShouldDo(ctx context.Context) (bool, error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step": s.Name(),
	})
	logger.Debug("checking if procedure should be run")

	exists, err := s.repo.Exists(ctx, s.vm.ID.Name(), s.vm.ID.Namespace())
	if err != nil {
		return false, fmt.Errorf("checking if spec exists: %w", err)
	}

	return exists, nil
}

// Do will perform the operation/procedure.
func (s *repoRelease) Do(ctx context.Context) ([]planner.Procedure, error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step": s.Name(),
	})
	logger.Debug("running step to release repo lease")

	if err := s.repo.ReleaseLease(ctx, s.vm); err != nil {
		return nil, fmt.Errorf("releasing lease for %s: %w", s.vm.ID, err)
	}

	return nil, nil
}
