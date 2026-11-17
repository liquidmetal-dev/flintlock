package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
)

const restartedDuringCaptureMsg = "flintlockd restarted during the capture"

// RecoverSnapshots fails every snapshot left in progress by a previous run of
// flintlockd, as no capture runs for it any more. It resumes the source of each
// if the capture left it paused, records the snapshot failed and removes every
// file of the snapshot but its record. It recovers every snapshot it can and
// returns the errors of the others.
func (a *app) RecoverSnapshots(ctx context.Context) error {
	snapshots, err := a.ports.SnapshotRepo.List(ctx, ports.SnapshotListOptions{})
	if err != nil {
		return fmt.Errorf("listing snapshots to recover: %w", err)
	}

	var errs []error

	for _, snapshot := range snapshots {
		if !snapshot.Status.Phase.InProgress() {
			continue
		}

		if err := a.recoverSnapshot(ctx, snapshot); err != nil {
			errs = append(errs, fmt.Errorf("recovering snapshot %s: %w", snapshot.UID, err))
		}
	}

	return errors.Join(errs...)
}

func (a *app) recoverSnapshot(ctx context.Context, snapshot *models.Snapshot) error {
	log.GetLogger(ctx).WithField("snapshot", snapshot.UID).
		Warnf("snapshot was %s when flintlockd stopped, failing it", snapshot.Status.Phase)

	var errs []error

	if err := a.resumeCaptureSource(ctx, snapshot); err != nil {
		errs = append(errs, err)
	}

	if err := a.setSnapshotPhase(ctx, snapshot, models.SnapshotPhaseFailed, restartedDuringCaptureMsg); err != nil {
		errs = append(errs, err)
	}

	if err := a.ports.SnapshotRepo.RemoveFiles(ctx, snapshot.UID); err != nil {
		errs = append(errs, fmt.Errorf("removing the files of snapshot %s: %w", snapshot.UID, err))
	}

	return errors.Join(errs...)
}

// resumeCaptureSource resumes the source of the snapshot when its VMM is paused.
// A source that its client wants paused is left paused.
func (a *app) resumeCaptureSource(ctx context.Context, snapshot *models.Snapshot) error {
	uid := snapshot.Source.UID()

	vm, err := a.ports.Repo.Get(ctx, ports.RepositoryGetOptions{UID: uid})
	if err != nil {
		return fmt.Errorf("getting source microvm %s: %w", uid, err)
	}

	if vm == nil || vm.Spec.WantsPaused() {
		return nil
	}

	provider, ok := a.ports.MicrovmProviders[vm.Spec.Provider]
	if !ok {
		return fmt.Errorf("microvm provider %s isn't available", vm.Spec.Provider)
	}

	a.vmLocks.Lock(uid)
	defer a.vmLocks.Unlock(uid)

	state, err := provider.State(ctx, vm.ID.String())
	if err != nil {
		return fmt.Errorf("getting the vmm state of microvm %s: %w", uid, err)
	}

	if state != ports.MicroVMStatePaused {
		return nil
	}

	if err := provider.Resume(ctx, vm); err != nil {
		return fmt.Errorf("resuming microvm %s: %w", uid, err)
	}

	log.GetLogger(ctx).WithField("snapshot", snapshot.UID).Infof("resumed source microvm %s", uid)

	return nil
}
