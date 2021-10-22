package controllers

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/weaveworks/flintlock/api/events"
	"github.com/weaveworks/flintlock/core/models"
	"github.com/weaveworks/flintlock/core/ports"
	"github.com/weaveworks/flintlock/pkg/defaults"
	"github.com/weaveworks/flintlock/pkg/log"
	"github.com/weaveworks/flintlock/pkg/queue"
)

func New(eventSvc ports.EventService, reconcileUC ports.ReconcileMicroVMsUseCase) *MicroVMController {
	return &MicroVMController{
		eventSvc:    eventSvc,
		reconcileUC: reconcileUC,
		queue:       queue.NewSimpleSyncQueue(),
	}
}

type MicroVMController struct {
	eventSvc    ports.EventService
	reconcileUC ports.ReconcileMicroVMsUseCase

	queue queue.Queue
}

func (r *MicroVMController) Run(ctx context.Context, numWorkers int, resyncPeriod time.Duration, resyncOnStart bool) error {
	logger := log.GetLogger(ctx).WithField("controller", "microvm")
	ctx = log.WithLogger(ctx, logger)
	logger.Infof("starting microvm controller with %d workers", numWorkers)

	go func() {
		<-ctx.Done()
		r.queue.Shutdown()
	}()

	if resyncOnStart {
		if err := r.resyncSpecs(ctx, logger); err != nil {
			// TODO: should we just log here?
			return fmt.Errorf("resyncing specs on start: %w", err)
		}
	}

	wg := &sync.WaitGroup{}
	logger.Info("starting event listener")
	wg.Add(1)
	go func() {
		defer wg.Done()
		r.runEventListener(ctx, resyncPeriod)
	}()

	logger.Info("Starting workers", "num_workers", numWorkers)
	wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go func() {
			defer wg.Done()
			for r.processQueueItem(ctx) {
			}
		}()
	}

	<-ctx.Done()
	logger.Info("Shutdown request received, waiting got children to finish")
	wg.Wait()
	logger.Info("All children finished")

	return nil
}

func (r *MicroVMController) runEventListener(ctx context.Context, resyncPeriod time.Duration) {
	logger := log.GetLogger(ctx)
	ticker := time.NewTicker(resyncPeriod)
	evtCh, errCh := r.eventSvc.SubscribeTopic(ctx, defaults.TopicMicroVMEvents)

	for {
		select {
		case <-ctx.Done():
			if cerr := ctx.Err(); cerr != nil && !errors.Is(cerr, context.Canceled) {
				logger.Errorf("cancelling event loop: %s", cerr)
			}

			return
		case evt := <-evtCh:
			if err := r.handleEvent(evt, logger); err != nil {
				logger.Errorf("handling events: %s", err)
				// TODO: should we exit here
			}
		case <-ticker.C:
			if err := r.resyncSpecs(ctx, logger); err != nil {
				logger.Errorf("resyncing specs: %s", err)
				// TODO: should we exit here
			}
		case evtErr := <-errCh:
			logger.Errorf("error from event service: %s", evtErr)
			// TODO: should we exit here?
		}
	}
}

func (r *MicroVMController) processQueueItem(ctx context.Context) bool {
	logger := log.GetLogger(ctx)
	item, shutdown := r.queue.Dequeue()
	if shutdown {
		return false
	}

	id, ok := item.(string)
	if !ok {
		logger.Errorf("vmid isn't a string, skipping %v", id)

		return true
	}
	vmid, err := models.NewVMIDFromString(id)
	if err != nil {
		logger.Errorf("failed to parse id into vmid %s, skipping: %s", id, err)

		return true
	}

	err = r.reconcileUC.ReconcileMicroVM(ctx, vmid.Name(), vmid.Namespace())
	if err != nil {
		logger.Errorf("failed to reconcile vmid %s: %s", vmid, err)
		r.queue.Enqueue(item)

		return true
	}

	return true
}

func (r *MicroVMController) handleEvent(envelope *ports.EventEnvelope, logger *logrus.Entry) error {
	var name, namespace string
	switch v := envelope.Event.(type) {
	case *events.MicroVMSpecCreated:
		created, _ := envelope.Event.(*events.MicroVMSpecCreated)
		name = created.ID
		namespace = created.Namespace
	case *events.MicroVMSpecDeleted:
		// Do not enqueue a deleted vmspec.
		// We can be smarter than this, but for now it's working
		// and we can reiterate on it.
		return nil
	case *events.MicroVMSpecUpdated:
		updated, _ := envelope.Event.(*events.MicroVMSpecUpdated)
		name = updated.ID
		namespace = updated.Namespace
	default:
		logger.Debugf("unhandled event type (%T) received", v)

		return nil
	}

	vmid, err := models.NewVMID(name, namespace)
	if err != nil {
		return fmt.Errorf("getting vmid from event data: %w", err)
	}

	logger.Debugf("enqueing vmid %s", vmid)
	r.queue.Enqueue(vmid.String())

	return nil
}

func (r *MicroVMController) resyncSpecs(ctx context.Context, logger *logrus.Entry) error {
	logger.Info("resyncing microvm specs")

	err := r.reconcileUC.ResyncMicroVMs(ctx, "")
	if err != nil {
		logger.Errorf("failed to resync microvms: %s", err)

		return fmt.Errorf("resyncing microvms: %w", err)
	}

	return nil
}
