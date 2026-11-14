// Package snapshot has the steps that use the snapshot store.
package snapshot

import (
	"context"
	"fmt"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/pkg/planner"
)

// NewAddRef returns the step that records that a microvm being restored reads
// the files of its snapshot, so that a delete of the snapshot keeps them.
func NewAddRef(vm *models.MicroVM, repo ports.SnapshotRepository) planner.Procedure {
	return &addRef{vm: vm, repo: repo}
}

type addRef struct {
	vm   *models.MicroVM
	repo ports.SnapshotRepository
}

// Name is the name of the procedure/operation.
func (s *addRef) Name() string {
	return "snapshot_add_ref"
}

func (s *addRef) ShouldDo(_ context.Context) (bool, error) {
	source := s.vm.Spec.SnapshotSource

	return source.RestorePending() && source.UsesSnapshotFiles(), nil
}

// Do will perform the operation/procedure.
func (s *addRef) Do(ctx context.Context) ([]planner.Procedure, error) {
	snapshotUID := s.vm.Spec.SnapshotSource.Snapshot.UID

	if err := s.repo.AddRef(ctx, snapshotUID, s.vm.ID.UID()); err != nil {
		return nil, fmt.Errorf("adding reference of microvm %s to snapshot %s: %w", s.vm.ID, snapshotUID, err)
	}

	return nil, nil
}

func (s *addRef) Verify(_ context.Context) error {
	return nil
}
