package application

import (
	"context"
	"fmt"

	"github.com/weaveworks/reignite/api/events"
	"github.com/weaveworks/reignite/core/models"
	"github.com/weaveworks/reignite/pkg/defaults"
	"github.com/weaveworks/reignite/pkg/log"
)

func (a *app) CreateMicroVM(ctx context.Context, mvm *models.MicroVM) (*models.MicroVM, error) {
	logger := log.GetLogger(ctx).WithField("component", "app")
	logger.Trace("creating microvm")

	if mvm == nil {
		return nil, errSpecRequired
	}

	if mvm.ID == "" {
		newID, err := a.idSvc.GenerateRandom()
		if err != nil {
			return nil, fmt.Errorf("generating random id for microvm: %w", err)
		}
		mvm.ID = newID
	}
	if mvm.Namespace == "" {
		mvm.Namespace = defaults.ContainerdNamespace // TODO: not sure this is correct
	}

	foundMvm, err := a.repo.Get(ctx, mvm.ID, mvm.Namespace)
	if err != nil {
		return nil, fmt.Errorf("checking to see if spec exists: %w", err)
	}
	if foundMvm != nil {
		return nil, errSpecAlreadyExists{
			name:      mvm.ID,
			namespace: mvm.Namespace,
		}
	}

	// TODO: validate the spec

	createdMVM, err := a.repo.Save(ctx, mvm)
	if err != nil {
		return nil, fmt.Errorf("saving microvm spec: %w", err)
	}

	if err := a.eventSvc.Publish(ctx, defaults.TopicMicroVMEvents, &events.MicroVMSpecCreated{
		ID:        mvm.ID,
		Namespace: mvm.Namespace,
	}); err != nil {
		return nil, fmt.Errorf("publishing microvm created event: %w", err)
	}

	return createdMVM, nil
}

func (a *app) UpdateMicroVM(ctx context.Context, mvm *models.MicroVM) (*models.MicroVM, error) {
	logger := log.GetLogger(ctx).WithField("component", "app")
	logger.Trace("updating microvm")

	if mvm == nil {
		return nil, errSpecRequired
	}

	foundMvm, err := a.repo.Get(ctx, mvm.ID, mvm.Namespace)
	if err != nil {
		return nil, fmt.Errorf("checking to see if spec exists: %w", err)
	}
	if foundMvm == nil {
		return nil, errSpecNotFound{
			name:      mvm.ID,
			namespace: mvm.Namespace,
		}
	}

	// TODO: validate incoming spec
	// TODO: check if update is valide (i.e. compare existing to requested update)

	updatedMVM, err := a.repo.Save(ctx, mvm)
	if err != nil {
		return nil, fmt.Errorf("updating microvm spec: %w", err)
	}

	if err := a.eventSvc.Publish(ctx, defaults.TopicMicroVMEvents, &events.MicroVMSpecUpdated{
		ID:        mvm.ID,
		Namespace: mvm.Namespace,
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

	foundMvm, err := a.repo.Get(ctx, id, namespace)
	if err != nil {
		return fmt.Errorf("checking to see if spec exists: %w", err)
	}
	if foundMvm == nil {
		logger.Infof("microvm %s/%s doesn't exist, skipping delete", id, namespace)

		return nil
	}

	err = a.repo.Delete(ctx, foundMvm)
	if err != nil {
		return fmt.Errorf("deleting microvm from repository: %w", err)
	}

	if err := a.eventSvc.Publish(ctx, defaults.TopicMicroVMEvents, &events.MicroVMSpecDeleted{
		ID:        id,
		Namespace: namespace,
	}); err != nil {
		return fmt.Errorf("publishing microvm deleted event: %w", err)
	}

	return nil
}
