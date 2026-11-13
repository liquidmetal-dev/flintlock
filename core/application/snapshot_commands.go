package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/liquidmetal-dev/flintlock/api/events"
	coreerrs "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
)

func (a *app) CreateSnapshot(ctx context.Context, req models.CreateSnapshotRequest) (*models.Snapshot, error) {
	logger := log.GetLogger(ctx).WithField("component", "app")
	logger.Debugf("creating snapshot of microvm %s", req.SourceUID)

	if req.SourceUID == "" {
		return nil, errUIDRequired
	}

	if req.After != "" && req.After != models.SnapshotAfterResume {
		return nil, coreerrs.NewFailedPrecondition(
			fmt.Sprintf("after %s is not supported: the source microvm can only be resumed", req.After),
		)
	}

	a.snapshotMu.Lock()
	defer a.snapshotMu.Unlock()

	mvm, err := a.ports.Repo.Get(ctx, ports.RepositoryGetOptions{UID: req.SourceUID})
	if err != nil {
		return nil, fmt.Errorf("getting source microvm %s: %w", req.SourceUID, err)
	}

	if mvm == nil {
		return nil, coreerrs.NewSpecNotFound("", "", "", req.SourceUID)
	}

	if err := a.checkSnapshotSource(mvm); err != nil {
		return nil, err
	}

	if err := a.checkNoSnapshotInProgress(ctx, mvm); err != nil {
		return nil, err
	}

	snapshot := &models.Snapshot{
		Name:      req.Name,
		Namespace: mvm.ID.Namespace(),
		Source:    mvm.ID,
		Labels:    req.Labels,
		CreatedAt: a.ports.Clock(),
	}

	if err := a.ports.SnapshotRepo.Create(ctx, snapshot); err != nil {
		return nil, fmt.Errorf("creating snapshot record: %w", err)
	}

	if err := a.ports.EventService.Publish(ctx, defaults.TopicSnapshotEvents, &events.SnapshotCreated{
		UID:       snapshot.UID,
		Name:      snapshot.Name,
		Namespace: snapshot.Namespace,
		SourceUID: snapshot.Source.UID(),
	}); err != nil {
		return nil, a.failUndispatchedSnapshot(ctx, snapshot, err)
	}

	return snapshot, nil
}

// failUndispatchedSnapshot marks a snapshot failed when its created event was
// not published. Nothing would capture the pending record, and it would refuse
// every later snapshot of the source. It runs without the request's
// cancellation, which may be the reason the publish failed.
func (a *app) failUndispatchedSnapshot(ctx context.Context, snapshot *models.Snapshot, publishErr error) error {
	err := fmt.Errorf("publishing snapshot created event for %s: %w", snapshot.UID, publishErr)

	snapshot.Status.Phase = models.SnapshotPhaseFailed
	snapshot.Status.Error = err.Error()

	if updateErr := a.ports.SnapshotRepo.Update(context.WithoutCancel(ctx), snapshot); updateErr != nil {
		return errors.Join(err, fmt.Errorf("marking snapshot %s failed: %w", snapshot.UID, updateErr))
	}

	return err
}

func (a *app) DeleteSnapshot(ctx context.Context, ref models.SnapshotRef) error {
	logger := log.GetLogger(ctx).WithField("component", "app")
	logger.Debugf("deleting snapshot %s", snapshotRefString(ref))

	snapshot, err := a.ports.SnapshotRepo.Get(ctx, ref)
	if err != nil {
		return fmt.Errorf("getting snapshot %s: %w", snapshotRefString(ref), err)
	}

	if snapshot.Status.Phase.InProgress() {
		return coreerrs.NewFailedPrecondition(fmt.Sprintf(
			"snapshot %s is %s: it can be deleted once it is ready or failed",
			snapshot.UID, snapshot.Status.Phase,
		))
	}

	if err := a.ports.SnapshotRepo.Delete(ctx, snapshot.UID); err != nil {
		return fmt.Errorf("deleting snapshot %s: %w", snapshot.UID, err)
	}

	if err := a.ports.EventService.Publish(ctx, defaults.TopicSnapshotEvents, &events.SnapshotDeleted{
		UID:       snapshot.UID,
		Name:      snapshot.Name,
		Namespace: snapshot.Namespace,
		SourceUID: snapshot.Source.UID(),
	}); err != nil {
		return fmt.Errorf("publishing snapshot deleted event: %w", err)
	}

	return nil
}

// checkSnapshotSource rejects a source microvm that cannot be snapshotted.
func (a *app) checkSnapshotSource(mvm *models.MicroVM) error {
	if mvm.Spec.DeletedAt != 0 || mvm.Status.State == models.DeletingState {
		return coreerrs.NewFailedPrecondition(fmt.Sprintf("microvm %s is being deleted", mvm.ID.UID()))
	}

	if mvm.Status.State != models.CreatedState {
		return coreerrs.NewFailedPrecondition(
			fmt.Sprintf("microvm %s is not running (state: %s)", mvm.ID.UID(), mvm.Status.State),
		)
	}

	provider, ok := a.ports.MicrovmProviders[mvm.Spec.Provider]
	if !ok {
		return coreerrs.NewFailedPrecondition(fmt.Sprintf("microvm provider %s isn't available", mvm.Spec.Provider))
	}

	if err := checkSnapshotCapability(provider); err != nil {
		return err
	}

	if err := checkSnapshotVolumes(mvm); err != nil {
		return err
	}

	if !mvm.Spec.AllowGuestAgent {
		return coreerrs.NewFailedPrecondition(
			fmt.Sprintf("microvm %s does not have allow_guest_agent set", mvm.ID.UID()),
		)
	}

	if !guestAgentReady(mvm) {
		return coreerrs.NewFailedPrecondition(
			fmt.Sprintf("the guest agent of microvm %s is not ready", mvm.ID.UID()),
		)
	}

	return nil
}

// checkSnapshotVolumes rejects a microvm with a device that a snapshot cannot capture.
// Only an overlay root can be captured.
func checkSnapshotVolumes(mvm *models.MicroVM) error {
	uid := mvm.ID.UID()

	if !mvm.Spec.RootVolume.IsOverlay() {
		return coreerrs.NewFailedPrecondition(fmt.Sprintf(
			"microvm %s has a root volume that is not an overlay: only an overlay root can be snapshotted", uid,
		))
	}

	for _, volume := range mvm.Spec.AdditionalVolumes {
		if volume.Source.VirtioFS != nil {
			return coreerrs.NewFailedPrecondition(fmt.Sprintf(
				"microvm %s has virtiofs volume %s: a virtiofs mount cannot be snapshotted", uid, volume.ID,
			))
		}
	}

	for _, volume := range mvm.Spec.AdditionalVolumes {
		if volume.Source.Container != nil && volume.EffectiveMode() == models.VolumeModeFull {
			return coreerrs.NewFailedPrecondition(fmt.Sprintf(
				"microvm %s has full volume %s: a full volume cannot be snapshotted", uid, volume.ID,
			))
		}
	}

	if len(mvm.Spec.AdditionalVolumes) > 0 {
		return coreerrs.NewFailedPrecondition(fmt.Sprintf(
			"microvm %s has additional volumes: a microvm with additional volumes cannot be snapshotted", uid,
		))
	}

	return nil
}

func guestAgentReady(mvm *models.MicroVM) bool {
	return mvm.Spec.AllowGuestAgent
}

// checkNoSnapshotInProgress rejects a second snapshot of a microvm while one is being taken.
func (a *app) checkNoSnapshotInProgress(ctx context.Context, mvm *models.MicroVM) error {
	snapshots, err := a.ports.SnapshotRepo.List(ctx, ports.SnapshotListOptions{
		Namespace: mvm.ID.Namespace(),
		SourceUID: mvm.ID.UID(),
	})
	if err != nil {
		return fmt.Errorf("listing snapshots of microvm %s: %w", mvm.ID.UID(), err)
	}

	for _, snapshot := range snapshots {
		if snapshot.Status.Phase.InProgress() {
			return coreerrs.NewFailedPrecondition(fmt.Sprintf(
				"microvm %s has snapshot %s in phase %s: only one snapshot at a time can be taken",
				mvm.ID.UID(), snapshot.UID, snapshot.Status.Phase,
			))
		}
	}

	return nil
}

func snapshotRefString(ref models.SnapshotRef) string {
	if ref.UID != "" {
		return ref.UID
	}

	return ref.Namespace + "/" + ref.Name
}
