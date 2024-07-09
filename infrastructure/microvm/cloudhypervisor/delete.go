package cloudhypervisor

import (
	"context"
	"errors"
	"fmt"
	"syscall"
	"time"

	"github.com/liquidmetal-dev/flintlock/pkg/cloudhypervisor"
	"github.com/liquidmetal-dev/flintlock/pkg/wait"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
	"github.com/liquidmetal-dev/flintlock/pkg/process"
	"github.com/sirupsen/logrus"
)

const (
	shutdownTimeOutSeconds  = 30
	shutdownCheckIntervalMS = 500
)

// Stop will stop a running microvm.
func (p *provider) Delete(ctx context.Context, id string) error {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"service": "cloudhypervisor_microvm",
		"vmid":    id,
	})
	logger.Info("deleting microvm")

	vmid, err := models.NewVMIDFromString(id)
	if err != nil {
		return fmt.Errorf("parsing vmid: %w", err)
	}

	vmState := NewState(*vmid, p.config.StateRoot, p.fs)

	pid, pidErr := vmState.PID()
	if pidErr != nil {
		return fmt.Errorf("unable to get PID: %w", pidErr)
	}

	processExists, err := process.Exists(pid)
	if err != nil {
		return fmt.Errorf("checking if cloud-hypervisor process is running: %w", err)
	}
	if !processExists {
		return nil
	}

	chClient := cloudhypervisor.New(vmState.SockPath())

	if shutdownErr := chClient.Shutdown(ctx); shutdownErr != nil {
		return fmt.Errorf("shutting down cloud-hypervisor vm: %w", shutdownErr)
	}

	shutdownFunc := func() (bool, error) {
		info, infoErr := chClient.Info(ctx)
		if infoErr != nil {
			return false, infoErr
		}

		return info.State == cloudhypervisor.VmStateShutdown || info.State == cloudhypervisor.VmStateCreated, nil
	}

	shutDownTimeout := shutdownTimeOutSeconds * time.Second
	checkInterval := shutdownCheckIntervalMS * time.Millisecond

	if waitErr := wait.ForCondition(shutdownFunc, shutDownTimeout, checkInterval); waitErr != nil {
		if !errors.Is(waitErr, wait.ErrWaitTimeout) {
			return fmt.Errorf("waiting for vm shutdown: %w", waitErr)
		}
	}

	logger.Debugf("sending SIGHUP to %d", pid)

	if sigErr := process.SendSignal(pid, syscall.SIGHUP); sigErr != nil {
		return fmt.Errorf("failed to terminate with SIGHUP: %w", sigErr)
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, p.deleteVMTimeout)
	defer cancel()

	// Make sure the microVM is stopped.
	if err := process.WaitWithContext(ctxTimeout, pid); err != nil {
		return fmt.Errorf("failed to wait for pid %d: %w", pid, err)
	}

	logger.Info("deleted microvm")

	return nil
}
