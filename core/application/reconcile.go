package application

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/weaveworks/reignite/api/events"
	"github.com/weaveworks/reignite/core/models"
	"github.com/weaveworks/reignite/core/plans"
	portsctx "github.com/weaveworks/reignite/core/ports/context"
	"github.com/weaveworks/reignite/pkg/defaults"
	"github.com/weaveworks/reignite/pkg/log"
	"github.com/weaveworks/reignite/pkg/planner"
)

func (a *app) ReconcileMicroVM(ctx context.Context, id, namespace string) error {
	logger := log.GetLogger(ctx).WithField("action", "reconcile")

	logger.Debugf("Getting spec for %s/%s", namespace, id)
	spec, err := a.ports.Repo.Get(ctx, id, namespace)
	if err != nil {
		return fmt.Errorf("getting microvm spec for reconcile: %w", err)
	}

	return a.reconcile(ctx, spec, logger)
}

func (a *app) ResyncMicroVMs(ctx context.Context, namespace string) error {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"action":    "resync",
		"namespace": "ns",
	})
	logger.Info("Resyncing specs")

	logger.Debug("Getting all specs")
	specs, err := a.ports.Repo.GetAll(ctx, namespace)
	if err != nil {
		return fmt.Errorf("getting all microvm specs for resync: %w", err)
	}

	for _, spec := range specs {
		if err := a.reconcile(ctx, spec, logger); err != nil {
			return fmt.Errorf("resync reconcile for spec %s: %w", spec.ID, err)
		}
	}

	return nil
}

func (a *app) plan(spec *models.MicroVM, logger *logrus.Entry) (planner.Plan, error) {
	l := logger.WithField("stage", "plan")
	l.Info("Generate plan")

	if spec.Status.Retry > a.cfg.MaximumRetry {
		return nil, reachedMaximumRetryError{vmid: spec.ID, retries: spec.Status.Retry}
	}

	// Delete only if the spec was marked as deleted.
	if spec.Spec.DeletedAt != 0 {
		input := &plans.DeletePlanInput{
			StateDirectory: a.cfg.RootStateDir,
			VM:             spec,
		}

		return plans.MicroVMDeletePlan(input), nil
	}

	// Create only if the state is Pending. Potentially we can retry later, but
	// that's maybe an Update as we may already have some resources and we only
	// have to run a few steps as an update. The other way is to add different
	// Failed states to handle Create retry, otherwise we can't tell if we have
	// to retry Update or Create. Delete is obvious because the DeletedAt field
	// is not zero.
	if spec.Status.State == models.PendingState {
		input := &plans.CreatePlanInput{
			StateDirectory: a.cfg.RootStateDir,
			VM:             spec,
		}

		return plans.MicroVMCreatePlan(input), nil
	}

	// Update plan.
	// If it's not a CreatePlan or a DeletePlan, we just check the state
	// and update.
	input := &plans.UpdatePlanInput{
		StateDirectory: a.cfg.RootStateDir,
		VM:             spec,
	}

	return plans.MicroVMUpdatePlan(input), nil
}

func (a *app) reconcile(ctx context.Context, spec *models.MicroVM, logger *logrus.Entry) error {
	l := logger.WithField("vmid", spec.ID.String())
	l.Info("Starting reconciliation")

	plan, planErr := a.plan(spec, l)
	if planErr != nil {
		return planErr
	}

	execCtx := portsctx.WithPorts(ctx, a.ports)

	executionID, err := a.ports.IdentifierService.GenerateRandom()
	if err != nil {
		return fmt.Errorf("generating plan execution id: %w", err)
	}

	actuator := planner.NewActuator()

	stepCount, err := actuator.Execute(execCtx, plan, executionID)
	if err != nil {
		return fmt.Errorf("executing plan: %w", err)
	}

	// Move this into a step at the same time as startvm is moved.
	if plan.Name() == plans.MicroVMDeletePlanName {
		if err := a.ports.EventService.Publish(ctx, defaults.TopicMicroVMEvents, &events.MicroVMSpecDeleted{
			ID:        spec.ID.Name(),
			Namespace: spec.ID.Namespace(),
		}); err != nil {
			return fmt.Errorf("publishing microvm updated event: %w", err)
		}

		if err := a.ports.Repo.ReleaseLease(ctx, spec); err != nil {
			return fmt.Errorf("releasing lease after plan execution: %w", err)
		}

		return nil
	}

	if stepCount == 0 {
		return nil
	}

	if _, err := a.ports.Repo.Save(ctx, spec); err != nil {
		return fmt.Errorf("saving spec after plan execution: %w", err)
	}

	// A little bit of hack until Update is implemented.
	//
	// Later move this into a step, so update and create can start if it's
	// not running.
	if plan.Name() == plans.MicroVMCreatePlanName {
		// if spec.Status.State == models.CreatedState {
		if err := a.ports.Provider.Start(ctx, spec.ID.String()); err != nil {
			return fmt.Errorf("starting micro vm %s: %w", spec.ID, err)
		}
		// }
	}

	return nil
}
