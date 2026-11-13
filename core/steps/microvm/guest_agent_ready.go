package microvm

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
	"github.com/liquidmetal-dev/flintlock/pkg/planner"
)

func NewGuestAgentReadyStep(
	vm *models.MicroVM,
	vmSvc ports.MicroVMService,
	agent ports.GuestAgentService,
	wait WaitConfig,
) planner.Procedure {
	return &guestAgentReadyStep{
		vm:    vm,
		vmSvc: vmSvc,
		agent: agent,
		wait:  wait,
	}
}

type guestAgentReadyStep struct {
	vm    *models.MicroVM
	vmSvc ports.MicroVMService
	agent ports.GuestAgentService
	wait  WaitConfig
}

// Name is the name of the procedure/operation.
func (s *guestAgentReadyStep) Name() string {
	return "microvm_guest_agent_ready"
}

// ShouldDo is false once GuestAgentReady is True or False, so that a guest
// without a running guest-agent does not hold each reconcile for the timeout.
func (s *guestAgentReadyStep) ShouldDo(_ context.Context) (bool, error) {
	if !s.vm.Spec.AllowGuestAgent {
		return false, nil
	}

	cond, ok := s.vm.Status.Conditions.Get(models.ConditionGuestAgentReady)

	return !ok || cond.Status == models.ConditionUnknown, nil
}

// Do pings the guest-agent until it answers or the timeout expires. It also
// checks the VMM on each attempt: on Firecracker the provider state is running
// as soon as the pid exists, so an exit during the boot shows only here.
func (s *guestAgentReadyStep) Do(ctx context.Context) ([]planner.Procedure, error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step": s.Name(),
		"vmid": s.vm.ID,
	})
	logger.Debug("waiting for the guest-agent to answer a ping")

	deadline := time.Now().Add(s.wait.Timeout)

	for {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("waiting for the guest-agent: %w", err)
		}

		running, err := s.checkVMM(ctx)
		if err != nil {
			return nil, err
		}

		if !running {
			return nil, errors.ErrUnableToBoot
		}

		pingErr := s.agent.Ping(ctx, s.vm.Status.VSockPath)
		if pingErr == nil {
			s.setReady(models.ConditionTrue, models.ReasonPingSucceeded, "")

			return nil, nil
		}

		if !time.Now().Before(deadline) {
			logger.WithError(pingErr).Warn("guest-agent did not answer a ping before the timeout")
			s.setReady(models.ConditionFalse, models.ReasonPingTimeout, pingErr.Error())

			return nil, nil
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("waiting for the guest-agent: %w", ctx.Err())
		case <-time.After(s.wait.Interval):
		}
	}
}

func (s *guestAgentReadyStep) Verify(_ context.Context) error {
	return nil
}

func (s *guestAgentReadyStep) checkVMM(ctx context.Context) (bool, error) {
	state, err := s.vmSvc.State(ctx, s.vm.ID.String())
	if err != nil {
		return false, fmt.Errorf("checking if microvm is running: %w", err)
	}

	if state != ports.MicroVMStateRunning {
		msg := fmt.Sprintf("vmm stopped, state %s", state)
		s.vm.Status.Conditions.Set(models.Condition{
			Type:    models.ConditionVMMRunning,
			Status:  models.ConditionFalse,
			Reason:  models.ReasonVMMExited,
			Message: msg,
		}, s.wait.now())
		s.setReady(models.ConditionFalse, models.ReasonVMMExited, msg)

		return false, nil
	}

	if _, ok := s.vm.Status.Conditions.Get(models.ConditionVMMRunning); !ok {
		s.vm.Status.Conditions.Set(models.Condition{
			Type:   models.ConditionVMMRunning,
			Status: models.ConditionTrue,
			Reason: models.ReasonRunning,
		}, s.wait.now())
	}

	return true, nil
}

func (s *guestAgentReadyStep) setReady(status models.ConditionStatus, reason, message string) {
	s.vm.Status.Conditions.Set(models.Condition{
		Type:    models.ConditionGuestAgentReady,
		Status:  status,
		Reason:  reason,
		Message: message,
	}, s.wait.now())
}
