package application

import (
	"context"
	"fmt"

	"github.com/weaveworks/reignite/api/events"
	coreerrs "github.com/weaveworks/reignite/core/errors"
	"github.com/weaveworks/reignite/core/models"
	"github.com/weaveworks/reignite/pkg/defaults"
	"github.com/weaveworks/reignite/pkg/log"
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
		vmid, err := models.NewVMID(name, defaults.MicroVMNamespace)
		if err != nil {
			return nil, fmt.Errorf("creating vmid: %w", err)
		}
		mvm.ID = *vmid
	}

	foundMvm, err := a.ports.Repo.Get(ctx, mvm.ID.Name(), mvm.ID.Namespace())
	if err != nil {
		if !coreerrs.IsSpecNotFound(err) {
			return nil, fmt.Errorf("checking to see if spec exists: %w", err)
		}
	}
	if foundMvm != nil {
		return nil, errSpecAlreadyExists{
			name:      mvm.ID.Name(),
			namespace: mvm.ID.Namespace(),
		}
	}

	// TODO: validate the spec

	createdMVM, err := a.ports.Repo.Save(ctx, mvm)
	if err != nil {
		return nil, fmt.Errorf("saving microvm spec: %w", err)
	}

	if err := a.ports.EventService.Publish(ctx, defaults.TopicMicroVMEvents, &events.MicroVMSpecCreated{
		ID:        mvm.ID.Name(),
		Namespace: mvm.ID.Namespace(),
	}); err != nil {
		return nil, fmt.Errorf("publishing microvm created event: %w", err)
	}

	return createdMVM, nil
}

func (a *app) UpdateMicroVM(ctx context.Context, mvm *models.MicroVM) (*models.MicroVM, error) {
	logger := log.GetLogger(ctx).WithField("component", "app")
	logger.Trace("updating microvm")

	if mvm == nil {
		return nil, coreerrs.ErrSpecRequired
	}
	if mvm.ID.IsEmpty() {
		return nil, coreerrs.ErrVMIDRequired
	}

	foundMvm, err := a.ports.Repo.Get(ctx, mvm.ID.Name(), mvm.ID.Namespace())
	if err != nil {
		return nil, fmt.Errorf("checking to see if spec exists: %w", err)
	}
	if foundMvm == nil {
		return nil, errSpecNotFound{
			name:      mvm.ID.Name(),
			namespace: mvm.ID.Namespace(),
		}
	}

	// TODO: validate incoming spec
	// TODO: check if update is valid (i.e. compare existing to requested update)

	updatedMVM, err := a.ports.Repo.Save(ctx, mvm)
	if err != nil {
		return nil, fmt.Errorf("updating microvm spec: %w", err)
	}

	if err := a.ports.EventService.Publish(ctx, defaults.TopicMicroVMEvents, &events.MicroVMSpecUpdated{
		ID:        mvm.ID.Name(),
		Namespace: mvm.ID.Namespace(),
	}); err != nil {
		return nil, fmt.Errorf("publishing microvm updated event: %w", err)
	}

	return updatedMVM, nil
}

func (a *app) DeleteMicroVM(ctx context.Context, id, namespace string) error {
	logger := log.GetLogger(ctx).WithField("component", "app")
	logger.Trace("deleting microvm")

	if id == "" {
		return errIDRequired
	}

	foundMvm, err := a.ports.Repo.Get(ctx, id, namespace)
	if err != nil {
		return fmt.Errorf("checking to see if spec exists: %w", err)
	}
	if foundMvm == nil {
		logger.Infof("microvm %s/%s doesn't exist, skipping delete", id, namespace)

		return nil
	}

	err = a.ports.Repo.Delete(ctx, foundMvm)
	if err != nil {
		return fmt.Errorf("deleting microvm from repository: %w", err)
	}

	if err := a.ports.EventService.Publish(ctx, defaults.TopicMicroVMEvents, &events.MicroVMSpecDeleted{
		ID:        id,
		Namespace: namespace,
	}); err != nil {
		return fmt.Errorf("publishing microvm deleted event: %w", err)
	}

	return nil
}
