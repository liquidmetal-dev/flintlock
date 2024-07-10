package containerd

import (
	"context"
	"fmt"

	"github.com/containerd/containerd/snapshots"
)

func snapshotKey(owner, ownerUsageID string) string {
	return fmt.Sprintf("flintlock/%s/%s", owner, ownerUsageID)
}

func snapshotExists(ctx context.Context, key string, ss snapshots.Snapshotter) (bool, error) {
	snapshotExists := false

	err := ss.Walk(ctx, func(_ context.Context, info snapshots.Info) error {
		if info.Name == key {
			snapshotExists = true
		}

		return nil
	})
	if err != nil {
		return false, fmt.Errorf("walking snapshots: %w", err)
	}

	return snapshotExists, nil
}
