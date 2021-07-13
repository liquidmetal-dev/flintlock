package lifecycle

import (
	"context"

	reignitev1 "github.com/weaveworks/reignite/api/reignite/v1alpha1"
	"github.com/weaveworks/reignite/pkg/microvm"
	"github.com/weaveworks/reignite/pkg/state"
)

// Manager is the definition of a lifecycle manager.
type Manager interface {
	// Create will create a new microvm.
	Create(ctx context.Context, microvm *reignitev1.MicroVM) error

	// Start will start the microvm with the supplied id.
	Start(ctx context.Context, id string) error

	// Delete will delete the microvm with the supplied id.
	Stop(ctx context.Context, id string) error
}

func New(vmState state.StateProvider, mvm microvm.Provider) (Manager, error) {
	return &microVMLifecycle{
		microVM: mvm,
		state:   vmState,
	}, nil
}

type microVMLifecycle struct {
	state   state.StateProvider
	microVM microvm.Provider
}
