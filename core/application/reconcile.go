package application

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/weaveworks/flintlock/api/events"
	"github.com/weaveworks/flintlock/core/models"
	"github.com/weaveworks/flintlock/core/plans"
	"github.com/weaveworks/flintlock/core/ports"
	portsctx "github.com/weaveworks/flintlock/core/ports/context"
	"github.com/weaveworks/flintlock/pkg/defaults"
	"github.com/weaveworks/flintlock/pkg/log"
	"github.com/weaveworks/flintlock/pkg/planner"
)

const backoffBaseInSeconds = 20

func (a *app) ReconcileMicroVM(ctx context.Context, id, namespace string) error {
	logger := log.GetLogger(ctx).WithField("action", "reconcile")

	logger.Debugf("Getting spec for %s/%s", namespace, id)

	spec, err := a.ports.Repo.Get(ctx, ports.RepositoryGetOptions{
		Name:      id,
		Namespace: namespace,
	})
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

	go func(id, ns string, sleepTime time.Duration) {
		time.Sleep(sleepTime)

		err := a.ports.EventService.Publish(
			context.Background(),
			defaults.TopicMicroVMEvents,
			&events.MicroVMSpecUpdated{
				ID:        id,
				Namespace: ns,
			},
		)
		if err != nil {
			logger.Errorf("failed to publish an update event for %s/%s", ns, id)
		}
	}(spec.ID.Name(), spec.ID.Namespace(), waitTime)

	return nil
}

func (a *app) reconcile(ctx context.Context, spec *models.MicroVM, logger *logrus.Entry) error {
	reconciliationID, err := a.ports.IdentifierService.GenerateRandom()
	if err != nil {
		return fmt.Errorf("generating reconciliationID id: %w", err)
	}

	localLogger := logger.WithFields(logrus.Fields{
		"reconciliation_id": reconciliationID,
		"vmid":              spec.ID.String(),
	})
	localLogger.Info("Starting reconciliation")

	if spec.Status.Retry > a.cfg.MaximumRetry {
		spec.Status.State = models.FailedState

		logger.Error(reachedMaximumRetryError{vmid: spec.ID, retries: spec.Status.Retry})

		return nil
	}

	if spec.Status.NotBefore > 0 && time.Now().Before(time.Unix(spec.Status.NotBefore, 0)) {
		return nil
	}

	plan := a.plan(spec, localLogger)
	execCtx := log.WithLogger(portsctx.WithPorts(ctx, a.ports), localLogger)
	actuator := planner.NewActuator()

	stepCount, err := actuator.Execute(execCtx, plan)
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

	if _, err := a.ports.Repo.Save(ctx, spec); err != nil {
		return fmt.Errorf("saving spec after plan execution: %w", err)
	}

	return nil
}
