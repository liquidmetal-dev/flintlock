package application

import (
	"context"
	"fmt"

	"github.com/weaveworks/flintlock/api/events"
	coreerrs "github.com/weaveworks/flintlock/core/errors"
	"github.com/weaveworks/flintlock/core/models"
	"github.com/weaveworks/flintlock/core/ports"
	"github.com/weaveworks/flintlock/pkg/defaults"
	"github.com/weaveworks/flintlock/pkg/log"
)

func (a *app) CreateMicroVM(ctx context.Context, mvm *models.MicroVM) (*models.MicroVM, error) {
	logger := log.GetLogger(ctx).WithField("component", "app")
	logger.Trace("creating microvm")

	if mvm == nil {
		return nil, coreerrs.ErrSpecRequired
	}

	if mvm.ID.IsEmpty() {
		name, err := a.ports.IdentifierService.GenerateRandom()
		if err != nil {
			return nil, fmt.Errorf("generating random name for microvm: %w", err)
		}

		vmid, err := models.NewVMID(name, defaults.MicroVMNamespace, "")
		if err != nil {
			return nil, fmt.Errorf("creating vmid: %w", err)
		}

		mvm.ID = *vmid
	}

	uid, err := a.ports.IdentifierService.GenerateRandom()
	if err != nil {
		return nil, fmt.Errorf("generating random ID for microvm: %w", err)
	}

	mvm.ID.SetUID(uid)

	foundMvm, err := a.ports.Repo.Get(ctx, ports.RepositoryGetOptions{
		Name:      mvm.ID.Name(),
		Namespace: mvm.ID.Namespace(),
		UID:       mvm.ID.UID(),
	})
	if err != nil {
		if !coreerrs.IsSpecNotFound(err) {
			return nil, fmt.Errorf("checking to see if spec exists: %w", err)
		}
	}

	if foundMvm != nil {
		return nil, specAlreadyExistsError{
			name:      mvm.ID.Name(),
			namespace: mvm.ID.Namespace(),
			uid:       mvm.ID.UID(),
		}
	}

	// Set the timestamp when the VMspec was created.
	mvm.Spec.CreatedAt = a.ports.Clock().Unix()
	mvm.Status.State = models.PendingState
	mvm.Status.Retry = 0

	createdMVM, err := a.ports.Repo.Save(ctx, mvm)
	if err != nil {
		return nil, fmt.Errorf("saving microvm spec: %w", err)
	}

	if err := a.ports.EventService.Publish(ctx, defaults.TopicMicroVMEvents, &events.MicroVMSpecCreated{
		ID:        mvm.ID.Name(),
		Namespace: mvm.ID.Namespace(),
		UID:       mvm.ID.UID(),
	}); err != nil {
		return nil, fmt.Errorf("publishing microvm created event: %w", err)
	}

	return createdMVM, nil
}

func (a *app) DeleteMicroVM(ctx context.Context, uid string) error {
	logger := log.GetLogger(ctx).WithField("component", "app")
	logger.Trace("deleting microvm")

	if uid == "" {
		return errUIDRequired
	}

	foundMvm, err := a.ports.Repo.Get(ctx, ports.RepositoryGetOptions{
		UID: uid,
	})
	if err != nil {
		return fmt.Errorf("checking to see if spec exists: %w", err)
	}

	if foundMvm == nil {
		return specNotFoundError{
			uid: uid,
		}
	}

	// Set the timestamp when the VMspec was deleted.
	foundMvm.Spec.DeletedAt = a.ports.Clock().Unix()
	foundMvm.Status.Retry = 0
	foundMvm.Status.State = models.DeletingState

	_, err = a.ports.Repo.Save(ctx, foundMvm)
	if err != nil {
		return fmt.Errorf("marking microvm spec for deletion: %w", err)
	}

	if err := a.ports.EventService.Publish(ctx, defaults.TopicMicroVMEvents, &events.MicroVMSpecUpdated{
		ID:        foundMvm.ID.Name(),
		Namespace: foundMvm.ID.Namespace(),
		UID:       foundMvm.ID.UID(),
	}); err != nil {
		return fmt.Errorf("publishing microvm updated event: %w", err)
	}

	return nil
}
