package application

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/weaveworks/flintlock/core/models"
	"github.com/weaveworks/flintlock/core/plans"
	"github.com/weaveworks/flintlock/core/ports"
	portsctx "github.com/weaveworks/flintlock/core/ports/context"
	"github.com/weaveworks/flintlock/pkg/log"
	"github.com/weaveworks/flintlock/pkg/planner"
)

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

	input := &plans.CreateOrUpdatePlanInput{
		StateDirectory: a.cfg.RootStateDir,
		VM:             spec,
	}

	return plans.MicroVMCreateOrUpdatePlan(input), nil
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

	if plan.Name() == plans.MicroVMDeletePlanName {
		return nil
	}

	if stepCount == 0 {
		return nil
	}

	if _, err := a.ports.Repo.Save(ctx, spec); err != nil {
		return fmt.Errorf("saving spec after plan execution: %w", err)
	}

	return nil
}
