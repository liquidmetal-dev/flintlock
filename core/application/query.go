package application

import (
	"context"
	"fmt"

	"github.com/weaveworks/flintlock/core/models"
	"github.com/weaveworks/flintlock/pkg/log"
)

func (a *app) GetMicroVM(ctx context.Context, id, namespace string) (*models.MicroVM, error) {
	logger := log.GetLogger(ctx).WithField("component", "app")
	logger.Tracef("querying microvm: %s, with namespace: %s", id, namespace)

	if id == "" {
		return nil, errIDRequired
	}

	if namespace == "" {
		return nil, errNamespaceRequired
	}

	foundMvm, err := a.ports.Repo.Get(ctx, id, namespace)
	if err != nil {
		return nil, fmt.Errorf("error attempting to locate microvm with id: %s, in namespace: %s: %w", id, namespace, err)
	}

	if foundMvm == nil {
		return nil, specNotFoundError{
			name:      id,
			namespace: namespace,
		}
	}

	return foundMvm, nil
}

func (a *app) GetAllMicroVM(ctx context.Context, namespace string) ([]*models.MicroVM, error) {
	return nil, errNotImplemeted
}
