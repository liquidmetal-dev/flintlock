package application

import (
	"context"
	"fmt"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
)

func (a *app) GetMicroVM(ctx context.Context, uid string) (*models.MicroVM, error) {
	logger := log.GetLogger(ctx).WithField("component", "app")
	logger.Tracef("querying microvm: %s", uid)

	if uid == "" {
		return nil, errUIDRequired
	}

	getOptions := ports.RepositoryGetOptions{
		UID: uid,
	}

	foundMvm, err := a.ports.Repo.Get(ctx, getOptions)
	if err != nil {
		return nil, fmt.Errorf("error attempting to locate microvm with uid: %s: %w", uid, err)
	}

	if foundMvm == nil {
		return nil, specNotFoundError{
			uid: uid,
		}
	}

	return foundMvm, nil
}

func (a *app) GetAllMicroVM(ctx context.Context, query models.ListMicroVMQuery) ([]*models.MicroVM, error) {
	logger := log.GetLogger(ctx).WithField("component", "app")
	logger.Tracef("querying all microvms: %v", query)

	foundMvms, err := a.ports.Repo.GetAll(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error attempting to list microvms: %v: %w", query, err)
	}

	if foundMvms == nil {
		logger.Tracef("no microvms were found: %v", query)

		return []*models.MicroVM{}, nil
	}

	return foundMvms, nil
}
