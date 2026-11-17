package application

import (
	"context"
	"fmt"

	"github.com/liquidmetal-dev/flintlock/api/events"
	coreerrs "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
)

// ReconcileSnapshot moves a pending snapshot to capturing, runs the capture and
// records the snapshot ready, or failed with the error of the capture. A failed
// capture is not retried, so it is not returned as an error.
func (a *app) ReconcileSnapshot(ctx context.Context, uid string) error {
	logger := log.GetLogger(ctx).WithField("action", "reconcile").WithField("snapshot", uid)

	snapshot, err := a.ports.SnapshotRepo.Get(ctx, models.SnapshotRef{UID: uid})
	if err != nil {
		if coreerrs.IsSnapshotNotFound(err) {
			logger.Debug("snapshot is deleted, nothing to capture")

			return nil
		}

		return fmt.Errorf("getting snapshot %s: %w", uid, err)
	}

	if snapshot.Status.Phase != models.SnapshotPhasePending {
		logger.Debugf("snapshot is %s, nothing to capture", snapshot.Status.Phase)

		return nil
	}

	if err := a.setSnapshotPhase(ctx, snapshot, models.SnapshotPhaseCapturing, ""); err != nil {
		return err
	}

	// The record must leave capturing even when the controller is stopping.
	finalCtx := context.WithoutCancel(ctx)

	if captureErr := a.CaptureSnapshot(ctx, snapshot); captureErr != nil {
		logger.WithError(captureErr).Error("snapshot capture failed")

		return a.setSnapshotPhase(finalCtx, snapshot, models.SnapshotPhaseFailed, captureErr.Error())
	}

	logger.Info("snapshot captured")

	return a.setSnapshotPhase(finalCtx, snapshot, models.SnapshotPhaseReady, "")
}

// setSnapshotPhase saves the phase and error of the snapshot and publishes the
// change. The record is the source of truth for the phase, so a failed publish
// is logged and does not fail the change.
func (a *app) setSnapshotPhase(
	ctx context.Context,
	snapshot *models.Snapshot,
	phase models.SnapshotPhase,
	errMsg string,
) error {
	snapshot.Status.Phase = phase
	snapshot.Status.Error = errMsg

	if err := a.ports.SnapshotRepo.Update(ctx, snapshot); err != nil {
		return fmt.Errorf("setting snapshot %s %s: %w", snapshot.UID, phase, err)
	}

	if err := a.ports.EventService.Publish(ctx, defaults.TopicSnapshotEvents, &events.SnapshotUpdated{
		UID:       snapshot.UID,
		Name:      snapshot.Name,
		Namespace: snapshot.Namespace,
		SourceUID: snapshot.Source.UID(),
		Phase:     string(phase),
		Error:     errMsg,
	}); err != nil {
		log.GetLogger(ctx).WithError(err).WithField("snapshot", snapshot.UID).
			Errorf("publishing snapshot updated event for phase %s", phase)
	}

	return nil
}
