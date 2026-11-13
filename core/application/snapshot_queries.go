package application

import (
	"context"
	"fmt"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
)

func (a *app) GetSnapshot(ctx context.Context, ref models.SnapshotRef) (*models.Snapshot, error) {
	logger := log.GetLogger(ctx).WithField("component", "app")
	logger.Tracef("querying snapshot: %s", snapshotRefString(ref))

	snapshot, err := a.ports.SnapshotRepo.Get(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("getting snapshot %s: %w", snapshotRefString(ref), err)
	}

	return snapshot, nil
}

func (a *app) GetAllSnapshots(ctx context.Context, options ports.SnapshotListOptions) ([]*models.Snapshot, error) {
	logger := log.GetLogger(ctx).WithField("component", "app")
	logger.Tracef("querying all snapshots: %v", options)

	snapshots, err := a.ports.SnapshotRepo.List(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("listing snapshots: %w", err)
	}

	if snapshots == nil {
		return []*models.Snapshot{}, nil
	}

	return snapshots, nil
}
