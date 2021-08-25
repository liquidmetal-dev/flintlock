package application

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/weaveworks/reignite/core/models"
	"github.com/weaveworks/reignite/core/plans"
	portsctx "github.com/weaveworks/reignite/core/ports/context"
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

func (a *app) reconcile(ctx context.Context, spec *models.MicroVM, logger *logrus.Entry) error {
	l := logger.WithField("vmid", spec.ID.String())
	l.Info("Starting reconciliation")

	input := &plans.CreatePlanInput{
		StateDirectory: a.cfg.RootStateDir,
		VM:             spec,
	}

	plan := plans.MicroVMCreatePlan(input)

	execCtx := portsctx.WithPorts(ctx, a.ports)

	executionID, err := a.ports.IdentifierService.GenerateRandom()
	if err != nil {
		return fmt.Errorf("generating plan execution id: %w", err)
	}

	actuator := planner.NewActuator()
	if err := actuator.Execute(execCtx, plan, executionID); err != nil {
		return fmt.Errorf("executing plan: %w", err)
	}

	if _, err := a.ports.Repo.Save(ctx, spec); err != nil {
		return fmt.Errorf("saving spec after plan execution: %w", err)
	}

	if err := a.ports.Provider.Start(ctx, spec.ID.String()); err != nil {
		return fmt.Errorf("starting micro vm %s: %w", spec.ID, err)
	}

	return nil
}
