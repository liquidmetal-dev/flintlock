package application

import (
	"context"
	"fmt"

	coreerrs "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
)

// powerChange describes a pause or a resume.
type powerChange struct {
	// verb names the change in log and error messages.
	verb string
	// notReady is the error message for a microvm in a state the change refuses.
	notReady string
	// fromVMM is the VMM state that the change acts on.
	fromVMM ports.MicroVMState
	// toVMM is the VMM state after the change. A VMM already in it needs no change.
	toVMM ports.MicroVMState
	// change makes the change through the provider.
	change func(ctx context.Context, provider ports.MicroVMService, vm *models.MicroVM) error
	// power is the power state the client wants after the change.
	power models.PowerState
	// state is the state of the microvm after the change.
	state models.MicroVMState
}

var (
	pauseChange = powerChange{
		verb:     "pausing",
		notReady: "is not running",
		fromVMM:  ports.MicroVMStateRunning,
		toVMM:    ports.MicroVMStatePaused,
		change: func(ctx context.Context, provider ports.MicroVMService, vm *models.MicroVM) error {
			return provider.Pause(ctx, vm)
		},
		power: models.PowerStatePaused,
		state: models.PausedState,
	}
	resumeChange = powerChange{
		verb:     "resuming",
		notReady: "is not paused",
		fromVMM:  ports.MicroVMStatePaused,
		toVMM:    ports.MicroVMStateRunning,
		change: func(ctx context.Context, provider ports.MicroVMService, vm *models.MicroVM) error {
			return provider.Resume(ctx, vm)
		},
		power: models.PowerStateRunning,
		state: models.CreatedState,
	}
)

// PauseMicroVM pauses the vCPUs of a running microvm and records that the client
// wants it paused, so that the reconciler keeps it paused. Pausing a paused
// microvm succeeds.
func (a *app) PauseMicroVM(ctx context.Context, uid string) error {
	return a.changePowerState(ctx, uid, pauseChange)
}

// ResumeMicroVM resumes a paused microvm and records that the client wants it
// running. Resuming a running microvm succeeds.
func (a *app) ResumeMicroVM(ctx context.Context, uid string) error {
	return a.changePowerState(ctx, uid, resumeChange)
}

// changePowerState pauses or resumes a created microvm. A microvm that is already
// in the state the change makes needs no change, so the change succeeds.
func (a *app) changePowerState(ctx context.Context, uid string, pc powerChange) error {
	logger := log.GetLogger(ctx).WithField("component", "app")
	logger.Debugf("%s microvm %s", pc.verb, uid)

	if uid == "" {
		return errUIDRequired
	}

	a.snapshotMu.Lock()
	defer a.snapshotMu.Unlock()

	mvm, err := a.ports.Repo.Get(ctx, ports.RepositoryGetOptions{UID: uid})
	if err != nil {
		return fmt.Errorf("getting microvm %s: %w", uid, err)
	}

	if mvm == nil {
		return coreerrs.NewSpecNotFound("", "", "", uid)
	}

	if mvm.Spec.DeletedAt != 0 || mvm.Status.State == models.DeletingState {
		return coreerrs.NewFailedPrecondition(fmt.Sprintf("microvm %s is being deleted", uid))
	}

	if mvm.Status.State != models.CreatedState && mvm.Status.State != models.PausedState {
		return coreerrs.NewFailedPrecondition(
			fmt.Sprintf("microvm %s %s (state: %s)", uid, pc.notReady, mvm.Status.State),
		)
	}

	// A snapshot pauses the microvm while it captures it, and resumes it after.
	if err := a.checkNoSnapshotInProgress(ctx, mvm); err != nil {
		return err
	}

	provider, ok := a.ports.MicrovmProviders[mvm.Spec.Provider]
	if !ok {
		return coreerrs.NewFailedPrecondition(fmt.Sprintf("microvm provider %s isn't available", mvm.Spec.Provider))
	}

	vmmState, err := provider.State(ctx, mvm.ID.String())
	if err != nil {
		return fmt.Errorf("getting the vmm state of microvm %s: %w", uid, err)
	}

	if vmmState != pc.fromVMM && vmmState != pc.toVMM {
		return coreerrs.NewFailedPrecondition(
			fmt.Sprintf("the vmm of microvm %s is not running (vmm state: %s)", uid, vmmState),
		)
	}

	if vmmState == pc.fromVMM {
		if err := pc.change(ctx, provider, mvm); err != nil {
			return fmt.Errorf("%s microvm %s: %w", pc.verb, uid, err)
		}
	}

	return a.savePowerState(ctx, mvm, pc.power, pc.state)
}

// savePowerState records the power state the client wants and the state of the microvm.
// It saves nothing when both are already recorded.
func (a *app) savePowerState(
	ctx context.Context,
	mvm *models.MicroVM,
	power models.PowerState,
	state models.MicroVMState,
) error {
	if mvm.Spec.WantsPaused() == (power == models.PowerStatePaused) && mvm.Status.State == state {
		return nil
	}

	mvm.Spec.PowerState = power
	mvm.Status.State = state

	if _, err := a.ports.Repo.Save(ctx, mvm); err != nil {
		return fmt.Errorf("saving the power state of microvm %s: %w", mvm.ID.UID(), err)
	}

	return nil
}
