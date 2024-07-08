package microvm

import (
	"context"
	"fmt"
	"time"

	"github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
	"github.com/liquidmetal-dev/flintlock/pkg/planner"
	"github.com/sirupsen/logrus"
)

func NewStartStep(
	vm *models.MicroVM,
	vmSvc ports.MicroVMService,
	bootWaitTimeSeconds int,
) planner.Procedure {
	return &startStep{
		vm:                  vm,
		vmSvc:               vmSvc,
		bootWaitTimeSeconds: bootWaitTimeSeconds,
	}
}

type startStep struct {
	vm                  *models.MicroVM
	vmSvc               ports.MicroVMService
	bootWaitTimeSeconds int
}

// Name is the name of the procedure/operation.
func (s *startStep) Name() string {
	return "microvm_start"
}

func (s *startStep) ShouldDo(ctx context.Context) (bool, error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step": s.Name(),
		"vmid": s.vm.ID,
	})
	logger.Debug("checking if procedure should be run")

	state, err := s.vmSvc.State(ctx, s.vm.ID.String())
	if err != nil {
		return false, fmt.Errorf("checking if microvm is running: %w", err)
	}

	return state != ports.MicroVMStateRunning, nil
}

// Do will perform the operation/procedure.
func (s *startStep) Do(ctx context.Context) ([]planner.Procedure, error) {
	if s.vm == nil {
		return nil, errors.ErrSpecRequired
	}

	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step": s.Name(),
		"vmid": s.vm.ID,
	})
	logger.Debug("starting microvm")

	if err := s.vmSvc.Start(ctx, s.vm); err != nil {
		return nil, fmt.Errorf("starting microvm: %w", err)
	}

	return nil, nil
}

func (s *startStep) Verify(ctx context.Context) error {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step": s.Name(),
		"vmid": s.vm.ID,
	})
	logger.Debug("waiting for the microvm to start")
	time.Sleep(time.Duration(s.bootWaitTimeSeconds) * time.Second)
	logger.Debug("verify microvm is started")

	state, err := s.vmSvc.State(ctx, s.vm.ID.String())
	if err != nil {
		return fmt.Errorf("checking if microvm is running: %w", err)
	}

	if state != ports.MicroVMStateRunning {
		return errors.ErrUnableToBoot
	}

	return nil
}
