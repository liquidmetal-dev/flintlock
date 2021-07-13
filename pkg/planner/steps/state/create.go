package state

import (
	"context"

	"github.com/sirupsen/logrus"

	reignitev1 "github.com/weaveworks/reignite/api/reignite/v1alpha1"
	"github.com/weaveworks/reignite/pkg/planner"
	"github.com/weaveworks/reignite/pkg/state"
)

func VMStateStep(microvm *reignitev1.MicroVM, vmState state.StateProvider, logger *logrus.Entry) planner.Procedure {
	return &microvmState{
		microvm: microvm,
		logger:  logger,
		vmState: vmState,
	}
}

type microvmState struct {
	microvm *reignitev1.MicroVM
	vmState state.StateProvider
	logger  *logrus.Entry
}

// Name is the name of the procedure/operation.
func (s *microvmState) Name() string {
	return "state_create"
}

// Do will perform the operation/procedure.
func (s *microvmState) Do(ctx context.Context) ([]planner.Procedure, error) {
	state := s.vmState.Get(s.microvm.Name)

	if err := state.Ensure(); err != nil {
		return nil, err
	}

	return nil, nil
}
