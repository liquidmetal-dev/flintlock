package microvm

import (
	"context"
	goerrors "errors"
	"fmt"
	"time"

	"github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
)

// WaitConfig configures how a step waits for a microvm.
type WaitConfig struct {
	// Timeout is how long the step waits in total.
	Timeout time.Duration
	// Interval is the time between two checks.
	Interval time.Duration
	// Clock gives the time for the conditions.
	Clock func() time.Time
}

func (c WaitConfig) now() time.Time {
	if c.Clock == nil {
		return time.Now()
	}

	return c.Clock()
}

// waitForRunning polls the provider state until the VMM runs.
func waitForRunning(
	ctx context.Context,
	vmSvc ports.MicroVMService,
	vmid string,
	cfg WaitConfig,
) (ports.MicroVMState, error) {
	deadline := time.Now().Add(cfg.Timeout)

	for {
		state, err := vmSvc.State(ctx, vmid)
		if err != nil {
			return state, fmt.Errorf("checking if microvm is running: %w", err)
		}

		if state == ports.MicroVMStateRunning {
			return state, nil
		}

		if !time.Now().Before(deadline) {
			return state, errors.ErrUnableToBoot
		}

		select {
		case <-ctx.Done():
			return state, ctx.Err() //nolint:wrapcheck // The caller checks for context errors.
		case <-time.After(cfg.Interval):
		}
	}
}

// verifyRunning waits for the VMM of vm and records the result in the
// VMMRunning condition.
func verifyRunning(ctx context.Context, vm *models.MicroVM, vmSvc ports.MicroVMService, cfg WaitConfig) error {
	state, err := waitForRunning(ctx, vmSvc, vm.ID.String(), cfg)

	switch {
	case err == nil:
		vm.Status.Conditions.Set(models.Condition{
			Type:   models.ConditionVMMRunning,
			Status: models.ConditionTrue,
			Reason: models.ReasonRunning,
		}, cfg.now())

		return nil
	case goerrors.Is(err, errors.ErrUnableToBoot):
		vm.Status.Conditions.Set(models.Condition{
			Type:    models.ConditionVMMRunning,
			Status:  models.ConditionFalse,
			Reason:  models.ReasonStartTimeout,
			Message: fmt.Sprintf("vmm not running after %s, last state %s", cfg.Timeout, state),
		}, cfg.now())

		return err
	default:
		return err
	}
}

// resetConditions marks the VMM and the guest-agent of a VMM that starts
// (again) as not known. It runs before the provider call, so that a failed
// start does not keep the conditions of the previous VMM.
func resetConditions(vm *models.MicroVM, cfg WaitConfig) {
	vm.Status.Conditions.Set(models.Condition{
		Type:   models.ConditionVMMRunning,
		Status: models.ConditionUnknown,
		Reason: models.ReasonVMMStarting,
	}, cfg.now())

	if !vm.Spec.AllowGuestAgent {
		return
	}

	vm.Status.Conditions.Set(models.Condition{
		Type:   models.ConditionGuestAgentReady,
		Status: models.ConditionUnknown,
		Reason: models.ReasonVMMStarting,
	}, cfg.now())
}
