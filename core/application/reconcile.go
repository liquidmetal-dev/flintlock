package application

import (
	"context"
	"fmt"
	"time"

	"github.com/liquidmetal-dev/flintlock/api/events"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/plans"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	portsctx "github.com/liquidmetal-dev/flintlock/core/ports/context"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
	"github.com/liquidmetal-dev/flintlock/pkg/planner"
	"github.com/sirupsen/logrus"
)

const backoffBaseInSeconds = 20

func (a *app) ReconcileMicroVM(ctx context.Context, vmid models.VMID) error {
	logger := log.GetLogger(ctx).WithField("action", "reconcile")

	logger.Debugf("Getting spec for %s", vmid.String())

	spec, err := a.ports.Repo.Get(ctx, ports.RepositoryGetOptions{
		Name:      vmid.Name(),
		Namespace: vmid.Namespace(),
		UID:       vmid.UID(),
	})
	if err != nil {
		return fmt.Errorf("getting microvm spec for reconcile: %w", err)
	}

	return a.reconcile(ctx, spec, logger)
}

func (a *app) plan(spec *models.MicroVM, logger *logrus.Entry) planner.Plan {
	l := logger.WithField("stage", "plan")
	l.Info("Generate plan")

	// Delete only if the spec was marked as deleted.
	if spec.Spec.DeletedAt != 0 {
		input := &plans.DeletePlanInput{
			StateDirectory: a.cfg.RootStateDir,
			VM:             spec,
		}

		return plans.MicroVMDeletePlan(input)
	}

	input := &plans.CreateOrUpdatePlanInput{
		StateDirectory: a.cfg.RootStateDir,
		VM:             spec,
	}

	return plans.MicroVMCreateOrUpdatePlan(input)
}

func (a *app) reschedule(ctx context.Context, logger *logrus.Entry, spec *models.MicroVM) error {
	spec.Status.Retry++
	waitTime := time.Duration(spec.Status.Retry*backoffBaseInSeconds) * time.Second
	spec.Status.NotBefore = time.Now().Add(waitTime).Unix()

	logger.Infof(
		"[%d/%d] reconciliation failed, rescheduled for next attempt at %s",
		spec.Status.Retry,
		a.cfg.MaximumRetry,
		time.Unix(spec.Status.NotBefore, 0),
	)

	if _, err := a.ports.Repo.Save(ctx, spec); err != nil {
		return fmt.Errorf("saving spec failed: %w", err)
	}

	go func(uid string, sleepTime time.Duration) {
		time.Sleep(sleepTime)

		err := a.ports.EventService.Publish(
			context.Background(),
			defaults.TopicMicroVMEvents,
			&events.MicroVMSpecUpdated{
				UID: uid,
			},
		)
		if err != nil {
			logger.Errorf("failed to publish an update event for %s", uid)
		}
	}(spec.ID.UID(), waitTime)

	return nil
}

func (a *app) reconcile(ctx context.Context, spec *models.MicroVM, logger *logrus.Entry) error {
	localLogger := logger.WithField("vmid", spec.ID.String())
	localLogger.Info("Starting reconciliation")

	plan := a.plan(spec, localLogger)

	if spec.Status.Retry > a.cfg.MaximumRetry {
		logger.Error(reachedMaximumRetryError{vmid: spec.ID, retries: spec.Status.Retry})

		return a.saveState(ctx, spec, plan, models.FailedState)
	}

	if spec.Status.NotBefore > 0 && time.Now().Before(time.Unix(spec.Status.NotBefore, 0)) {
		return nil
	}

	execCtx := portsctx.WithPorts(ctx, a.ports)

	executionID, err := a.ports.IdentifierService.GenerateRandom()
	if err != nil {
		if scheduleErr := a.reschedule(ctx, localLogger, spec); scheduleErr != nil {
			return fmt.Errorf("rescheduling failed: %w", scheduleErr)
		}

		return fmt.Errorf("generating plan execution id: %w", err)
	}

	actuator := planner.NewActuator()

	stepCount, err := actuator.Execute(execCtx, plan, executionID)
	if err != nil {
		if scheduleErr := a.reschedule(ctx, localLogger, spec); scheduleErr != nil {
			return fmt.Errorf("rescheduling failed: %w", scheduleErr)
		}

		return fmt.Errorf("executing plan: %w", err)
	}

	if plan.Name() == plans.MicroVMDeletePlanName {
		return nil
	}

	if stepCount == 0 {
		return nil
	}

	spec.Status.Retry = 0
	spec.Status.NotBefore = 0

	return a.saveState(ctx, spec, plan, models.CreatedState)
}

func (a *app) saveState(ctx context.Context, spec *models.MicroVM, plan planner.Plan, state models.MicroVMState) error {
	plan.Finalise(state)

	if _, err := a.ports.Repo.Save(ctx, spec); err != nil {
		return fmt.Errorf("saving spec after plan execution: %w", err)
	}

	return nil
}
