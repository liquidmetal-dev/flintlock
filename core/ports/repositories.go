package ports

import (
	"context"

	"github.com/weaveworks/reignite/core/models"
)

// MicroVMRepository is the port definition for a microvm repository.
type MicroVMRepository interface {
	// Save will save the supplied microvm spec.
	Save(ctx context.Context, microvm *models.MicroVM) (*models.MicroVM, error)
	// Delete will delete the supplied microvm.
	Delete(ctx context.Context, microvm *models.MicroVM) error
	// Get will get the microvm spec with the given name/namespace.
	Get(ctx context.Context, name, namespace string) (*models.MicroVM, error)
	// GetAll will get a list of microvm details. If namespace is an empty string all
	// details of microvms will be returned.
	GetAll(ctx context.Context, namespace string) ([]*models.MicroVM, error)
	// Exists checks to see if the microvm spec exists in the repo.
	Exists(ctx context.Context, name, namespace string) (bool, error)
	// ReleaseLEase will release the supplied lease.
	ReleaseLease(ctx context.Context, microvm *models.MicroVM) error
}
