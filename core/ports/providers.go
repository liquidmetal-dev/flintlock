package ports

import (
	"context"

	"github.com/weaveworks/reignite/core/models"
)

// MicroVMProvider is the port definition for a microvm provider.
type MicroVMProvider interface {
	// Capabilities returns a list of the capabilities the provider supports.
	Capabilities() models.Capabilities

	// CreateVM will create a new microvm.
	CreateVM(ctx context.Context, vm *models.MicroVM) (*models.MicroVM, error)
	// StartVM will start a created microvm.
	StartVM(ctx context.Context, id string) error
	// PauseVM will pause a started microvm.
	PauseVM(ctx context.Context, id string) error
	// ResumeVM will resume a paused microvm.
	ResumeVM(ctx context.Context, id string) error
	// StopVM will stop a paused or running microvm.
	StopVM(ctx context.Context, id string) error
	// DeleteVM will delete a VM and its runtime state.
	DeleteVM(ctx context.Context, id string) error

	// ListVMs will return a list of the microvms.
	ListVMs(ctx context.Context, count int) ([]*models.MicroVM, error)
}
