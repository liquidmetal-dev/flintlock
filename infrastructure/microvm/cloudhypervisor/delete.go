package cloudhypervisor

import (
	"context"
	"errors"
	"fmt"
	"syscall"
	"time"

	"github.com/weaveworks-liquidmetal/flintlock/pkg/cloudhypervisor"
	"github.com/weaveworks-liquidmetal/flintlock/pkg/wait"

	"github.com/sirupsen/logrus"
	"github.com/weaveworks-liquidmetal/flintlock/core/models"
	"github.com/weaveworks-liquidmetal/flintlock/pkg/log"
	"github.com/weaveworks-liquidmetal/flintlock/pkg/process"
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
	err = chClient.Shutdown(ctx)
	if err != nil {
		return fmt.Errorf("shutting down cloud-hypervisor vm: %w", err)
	}

	shutdownFunc := func() (bool, error) {
		info, infoErr := chClient.Info(ctx)
		if infoErr != nil {
			return false, err
		}

		return info.State == cloudhypervisor.VmStateShutdown || info.State == cloudhypervisor.VmStateCreated, nil
	}
	shutDownTimeout := 30 * time.Second
	checkInterval := 500 * time.Millisecond
	if waitErr := wait.ForCondition(shutdownFunc, shutDownTimeout, checkInterval); waitErr != nil {
		if !errors.Is(waitErr, wait.ErrWaitTimeout) {
			return fmt.Errorf("waiting for vm shutdown: %w", err)
		}
	}

	logger.Infof("sending SIGHUP to %d", pid)

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
