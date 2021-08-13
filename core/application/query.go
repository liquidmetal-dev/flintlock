package application

import (
	"context"

	"github.com/weaveworks/reignite/core/models"
)

func (a *app) GetMicroVM(ctx context.Context, id, namespace string) (*models.MicroVM, error) {
	return nil, nil
}

func (a *app) GetAllMicroVM(ctx context.Context, namespace string) ([]*models.MicroVM, error) {
	return nil, nil
}
