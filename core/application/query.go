package application

import (
	"context"

	"github.com/weaveworks/flintlock/core/models"
)

func (a *app) GetMicroVM(ctx context.Context, id, namespace string) (*models.MicroVM, error) {
	return nil, errNotImplemeted
}

func (a *app) GetAllMicroVM(ctx context.Context, namespace string) ([]*models.MicroVM, error) {
	return nil, errNotImplemeted
}
