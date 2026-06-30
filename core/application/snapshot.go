package application

import (
	"context"
	"fmt"

	coreerrs "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
)

func (a *app) SnapshotMicroVM(ctx context.Context, uid, reference string) (*ports.SnapshotImage, error) {
	logger := log.GetLogger(ctx).WithField("component", "app")
	logger.Trace("snapshotting microvm")

	if uid == "" {
		return nil, errUIDRequired
	}

	if reference == "" {
		return nil, errSnapshotReferenceRequired
	}

	foundMvm, err := a.ports.Repo.Get(ctx, ports.RepositoryGetOptions{
		UID: uid,
	})
	if err != nil {
		return nil, fmt.Errorf("checking to see if spec exists: %w", err)
	}

	if foundMvm == nil {
		return nil, specNotFoundError{
			uid: uid,
		}
	}

	provider, ok := a.ports.MicrovmProviders[foundMvm.Spec.Provider]
	if !ok {
		return nil, fmt.Errorf("microvm provider %s isn't available", foundMvm.Spec.Provider)
	}

	if !provider.Capabilities().Has(models.SnapshotCapability) {
		return nil, coreerrs.NewNotSupported("snapshot")
	}

	logger = logger.WithField("vmid", foundMvm.ID)
	logger.Infof("taking snapshot of microvm into %s", reference)

	result, err := provider.Snapshot(ctx, ports.SnapshotInput{VMID: foundMvm.ID})
	if err != nil {
		return nil, fmt.Errorf("taking snapshot of microvm: %w", err)
	}

	image, err := a.ports.SnapshotPackager.Build(ctx, ports.SnapshotPackageInput{
		Reference: reference,
		Artifacts: result.Artifacts,
		Spec:      foundMvm,
	})
	if err != nil {
		return nil, fmt.Errorf("packaging snapshot into image: %w", err)
	}

	// The OCI image is now the durable artifact; clean up the raw scratch files.
	if result.Directory != "" {
		if rmErr := a.ports.FileSystem.RemoveAll(result.Directory); rmErr != nil {
			logger.Warnf("failed to clean up snapshot scratch dir %s: %s", result.Directory, rmErr)
		}
	}

	return image, nil
}
