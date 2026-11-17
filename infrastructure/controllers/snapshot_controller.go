package controllers

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/liquidmetal-dev/flintlock/api/events"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
	"github.com/liquidmetal-dev/flintlock/pkg/queue"
)

var errNoSnapshotWorkers = errors.New("the snapshot controller needs at least 1 worker")

// NewSnapshotController returns a controller that captures pending snapshots.
func NewSnapshotController(
	eventSvc ports.EventService,
	reconcileUC ports.ReconcileSnapshotsUseCase,
	queryUC ports.SnapshotQueryUseCases,
) *SnapshotController {
	return &SnapshotController{
		eventSvc:    eventSvc,
		reconcileUC: reconcileUC,
		queryUC:     queryUC,
		queue:       queue.NewSimpleSyncQueue(),
	}
}

// SnapshotController captures the snapshots that are pending, at most as many at
// a time as it has workers.
type SnapshotController struct {
	eventSvc    ports.EventService
	reconcileUC ports.ReconcileSnapshotsUseCase
	queryUC     ports.SnapshotQueryUseCases

	queue queue.Queue
}

// Run recovers the snapshots that a previous run left in progress, and then
// captures pending snapshots with numWorkers workers until ctx is done. Every
// resyncPeriod it queues the pending snapshots again, so that a snapshot whose
// created event was missed is still captured.
func (r *SnapshotController) Run(ctx context.Context, numWorkers int, resyncPeriod time.Duration) error {
	if numWorkers < 1 {
		return errNoSnapshotWorkers
	}

	logger := log.GetLogger(ctx).WithField("controller", "snapshot")
	ctx = log.WithLogger(ctx, logger)
	logger.Infof("starting snapshot controller with %d workers", numWorkers)

	go func() {
		<-ctx.Done()
		r.queue.Shutdown()
	}()

	// Subscribe before the recovery, so that no snapshot created meanwhile is missed.
	evtCh, errCh := r.eventSvc.SubscribeTopic(ctx, defaults.TopicSnapshotEvents)

	if err := r.reconcileUC.RecoverSnapshots(ctx); err != nil {
		// The snapshots that were recovered must not wait for the others.
		logger.Errorf("recovering snapshots on start: %s", err)
	}

	if err := r.resyncPending(ctx, logger); err != nil {
		logger.Errorf("resyncing snapshots on start: %s", err)
	}

	wg := &sync.WaitGroup{}

	wg.Add(1)

	go func() {
		defer wg.Done()
		r.runEventListener(ctx, evtCh, errCh, resyncPeriod)
	}()

	wg.Add(numWorkers)

	for range numWorkers {
		go func() {
			defer wg.Done()

			for r.processQueueItem(ctx) {
			}
		}()
	}

	<-ctx.Done()
	logger.Info("Shutdown request received, waiting for children to finish")
	wg.Wait()
	logger.Info("All children finished")

	return nil
}

func (r *SnapshotController) runEventListener(
	ctx context.Context,
	evtCh <-chan *ports.EventEnvelope,
	errCh <-chan error,
	resyncPeriod time.Duration,
) {
	logger := log.GetLogger(ctx)
	ticker := time.NewTicker(resyncPeriod)

	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if cerr := ctx.Err(); cerr != nil && !errors.Is(cerr, context.Canceled) {
				logger.Errorf("cancelling event loop: %s", cerr)
			}

			return
		case evt, ok := <-evtCh:
			if !ok {
				evtCh = nil

				continue
			}

			r.handleEvent(evt, logger)
		case <-ticker.C:
			if err := r.resyncPending(ctx, logger); err != nil {
				logger.Errorf("resyncing snapshots: %s", err)
			}
		case evtErr := <-errCh:
			logger.Errorf("error from event service: %s", evtErr)
		}
	}
}

func (r *SnapshotController) handleEvent(envelope *ports.EventEnvelope, logger *logrus.Entry) {
	created, ok := envelope.Event.(*events.SnapshotCreated)
	if !ok {
		logger.Debugf("unhandled event type (%T) received", envelope.Event)

		return
	}

	logger.Debugf("enqueing snapshot %s", created.UID)
	r.queue.Enqueue(created.UID)
}

// processQueueItem captures the next snapshot of the queue. A snapshot that fails
// to reconcile is not queued again: it is either still pending, and the next
// resync queues it, or no longer something a retry can change.
func (r *SnapshotController) processQueueItem(ctx context.Context) bool {
	logger := log.GetLogger(ctx)

	item, shutdown := r.queue.Dequeue()
	if shutdown {
		return false
	}

	uid, ok := item.(string)
	if !ok {
		logger.Errorf("snapshot uid isn't a string, skipping %v", item)

		return true
	}

	if err := r.reconcileUC.ReconcileSnapshot(ctx, uid); err != nil {
		logger.Errorf("failed to reconcile snapshot %s: %s", uid, err)
	}

	return true
}

func (r *SnapshotController) resyncPending(ctx context.Context, logger *logrus.Entry) error {
	logger.Debug("resyncing pending snapshots")

	snapshots, err := r.queryUC.GetAllSnapshots(ctx, ports.SnapshotListOptions{})
	if err != nil {
		return fmt.Errorf("getting all snapshots for resync: %w", err)
	}

	for _, snapshot := range snapshots {
		if snapshot.Status.Phase != models.SnapshotPhasePending {
			continue
		}

		logger.Debugf("enqueing snapshot %s for resync", snapshot.UID)
		r.queue.Enqueue(snapshot.UID)
	}

	return nil
}
