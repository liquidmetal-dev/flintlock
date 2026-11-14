package application

import (
	"context"
	"fmt"
	"time"

	coreerrs "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/plans"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	portsctx "github.com/liquidmetal-dev/flintlock/core/ports/context"
	"github.com/liquidmetal-dev/flintlock/internal/version"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
)

func (a *app) CaptureSnapshot(ctx context.Context, snapshot *models.Snapshot) error {
	logger := log.GetLogger(ctx).WithField("component", "app")
	logger.Debugf("capturing snapshot %s of microvm %s", snapshot.UID, snapshot.Source.UID())

	vm, err := a.ports.Repo.Get(ctx, ports.RepositoryGetOptions{UID: snapshot.Source.UID()})
	if err != nil {
		return fmt.Errorf("getting source microvm %s: %w", snapshot.Source.UID(), err)
	}

	if vm == nil {
		return coreerrs.NewSpecNotFound("", "", "", snapshot.Source.UID())
	}

	executionID, err := a.ports.IdentifierService.GenerateRandom()
	if err != nil {
		return fmt.Errorf("generating plan execution id: %w", err)
	}

	input := &plans.SnapshotCapturePlanInput{
		Snapshot:         snapshot,
		VM:               vm,
		PauseBudget:      a.snapshotMaxPause(),
		Locks:            a.vmLocks,
		FlintlockVersion: version.Version,
	}

	return plans.RunSnapshotCapture(portsctx.WithPorts(ctx, a.ports), input, executionID)
}

func (a *app) snapshotMaxPause() time.Duration {
	if a.cfg.SnapshotMaxPause > 0 {
		return a.cfg.SnapshotMaxPause
	}

	return defaults.SnapshotMaxPause
}
