package microvm

import (
	"context"
	goerrors "errors"
	"fmt"

	"github.com/liquidmetal-dev/flintlock/api/events"
	"github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
	"github.com/liquidmetal-dev/flintlock/pkg/planner"
)

var errGuestAgentNotReadyAfterRestore = goerrors.New("the guest agent did not answer after the restore")

// NewRestoredStep returns the last step of a restore. It marks the snapshot
// source restored, so that the microvm is never restored again, and publishes
// the restored event.
func NewRestoredStep(vm *models.MicroVM, eventSvc ports.EventService) planner.Procedure {
	return &restoredStep{
		vm:       vm,
		eventSvc: eventSvc,
	}
}

type restoredStep struct {
	vm       *models.MicroVM
	eventSvc ports.EventService
}

// Name is the name of the procedure/operation.
func (s *restoredStep) Name() string {
	return "microvm_restored"
}

func (s *restoredStep) ShouldDo(_ context.Context) (bool, error) {
	return s.vm.Spec.SnapshotSource.RestorePending(), nil
}

// Do fails while the guest agent is not ready: a restored microvm counts as
// created only once its guest agent has bound the vsock again.
func (s *restoredStep) Do(ctx context.Context) ([]planner.Procedure, error) {
	if s.vm == nil || s.vm.Spec.SnapshotSource == nil {
		return nil, errors.ErrSpecRequired
	}

	cond, ok := s.vm.Status.Conditions.Get(models.ConditionGuestAgentReady)
	if !ok || cond.Status != models.ConditionTrue {
		return nil, errGuestAgentNotReadyAfterRestore
	}

	source := s.vm.Spec.SnapshotSource
	source.Restored = true

	log.GetLogger(ctx).WithField("step", s.Name()).
		Infof("restored microvm %s from snapshot %s, generation %d", s.vm.ID, source.Snapshot.UID, source.Generation)

	if err := s.eventSvc.Publish(ctx, defaults.TopicMicroVMEvents, &events.MicroVMRestored{
		ID:          s.vm.ID.Name(),
		Namespace:   s.vm.ID.Namespace(),
		UID:         s.vm.ID.UID(),
		SnapshotUID: source.Snapshot.UID,
		Generation:  source.Generation,
	}); err != nil {
		return nil, fmt.Errorf("publishing microvm restored event: %w", err)
	}

	return nil, nil
}

func (s *restoredStep) Verify(_ context.Context) error {
	return nil
}
