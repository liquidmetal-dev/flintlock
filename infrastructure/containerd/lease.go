package containerd

import (
	"context"
	"fmt"

	"github.com/containerd/containerd/leases"
)

func withOwnerLease(ctx context.Context, owner string, client Client) (context.Context, error) {
	leaseName := getLeaseNameForOwner(owner)

	l, err := getExistingOrCreateLease(ctx, leaseName, client.LeasesService())
	if err != nil {
		return nil, fmt.Errorf("getting containerd lease: %w", err)
	}

	return leases.WithLease(ctx, l.ID), nil
}

func getExistingOrCreateLease(ctx context.Context, name string, manager leases.Manager) (*leases.Lease, error) {
	filter := fmt.Sprintf("id==%s", name)

	existingLeases, err := manager.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("listing existing containerd leases: %w", err)
	}

	for _, lease := range existingLeases {
		if lease.ID == name {
			return &lease, nil
		}
	}

	lease, err := manager.Create(ctx, leases.WithID(name))
	if err != nil {
		return nil, fmt.Errorf("creating lease with name %s: %w", name, err)
	}

	return &lease, nil
}

func deleteLease(ctx context.Context, owner string, client Client) error {
	leaseName := getLeaseNameForOwner(owner)
	lease := leases.Lease{ID: leaseName}

	err := client.LeasesService().Delete(ctx, lease, leases.SynchronousDelete)
	if err != nil {
		return fmt.Errorf("delete lease %s: %w", leaseName, err)
	}

	return nil
}

func getLeaseNameForOwner(owner string) string {
	return fmt.Sprintf("flintlock/%s", owner)
}
