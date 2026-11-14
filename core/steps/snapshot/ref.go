// Package snapshot has the steps that use the snapshot store.
package snapshot

import (
	"context"
	"fmt"

	"github.com/spf13/afero"

	cerrs "github.com/liquidmetal-dev/flintlock/core/errors"
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

// NewReleaseRef returns the step that removes the reference of a microvm to
// the files of its snapshot. It runs while the state directory of the microvm,
// vmDir, exists.
func NewReleaseRef(vm *models.MicroVM, repo ports.SnapshotRepository, vmDir string, fs afero.Fs) planner.Procedure {
	return &releaseRef{vm: vm, repo: repo, vmDir: vmDir, fs: fs}
}

type releaseRef struct {
	vm    *models.MicroVM
	repo  ports.SnapshotRepository
	vmDir string
	fs    afero.Fs
}

// Name is the name of the procedure/operation.
func (s *releaseRef) Name() string {
	return "snapshot_release_ref"
}

func (s *releaseRef) ShouldDo(_ context.Context) (bool, error) {
	if !s.vm.Spec.SnapshotSource.UsesSnapshotFiles() {
		return false, nil
	}

	exists, err := afero.DirExists(s.fs, s.vmDir)
	if err != nil {
		return false, fmt.Errorf("checking if dir %s exists: %w", s.vmDir, err)
	}

	return exists, nil
}

// Do will perform the operation/procedure. A snapshot that is gone has no
// reference left to release.
func (s *releaseRef) Do(ctx context.Context) ([]planner.Procedure, error) {
	snapshotUID := s.vm.Spec.SnapshotSource.Snapshot.UID

	err := s.repo.ReleaseRef(ctx, snapshotUID, s.vm.ID.UID())
	if err != nil && !cerrs.IsSnapshotNotFound(err) {
		return nil, fmt.Errorf("releasing reference of microvm %s to snapshot %s: %w", s.vm.ID, snapshotUID, err)
	}

	return nil, nil
}

func (s *releaseRef) Verify(_ context.Context) error {
	return nil
}
