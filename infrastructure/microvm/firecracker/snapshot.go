package firecracker

import (
	"context"
	"errors"
	"fmt"

	sdk "github.com/firecracker-microvm/firecracker-go-sdk"
	fcmodels "github.com/firecracker-microvm/firecracker-go-sdk/client/models"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"

	cerrs "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
	"github.com/liquidmetal-dev/flintlock/pkg/process"
)

// Snapshot pauses a running microvm, captures a full snapshot to disk and resumes it.
func (p *fcProvider) Snapshot(ctx context.Context, input ports.SnapshotInput) (result *ports.SnapshotResult, retErr error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"service": "firecracker_microvm",
		"vmid":    input.VMID.String(),
	})
	logger.Info("snapshotting microvm")

	vmState := NewState(input.VMID, p.config.StateRoot, p.fs)

	// Pre-flight: only a running VM can be snapshotted.
	running, err := p.isRunning(vmState)
	if err != nil {
		return nil, err
	}

	if !running {
		return nil, fmt.Errorf("cannot snapshot: %w", cerrs.ErrNotRunning)
	}

	scratch := vmState.Root() + "/snapshot"
	if err := p.fs.MkdirAll(scratch, defaults.DataDirPerm); err != nil {
		return nil, fmt.Errorf("creating snapshot scratch dir %s: %w", scratch, err)
	}

	statePath := scratch + "/vmstate"
	memPath := scratch + "/memory"

	fcClient := sdk.NewClient(vmState.SockPath(), logger, false)

	pausedState := fcmodels.VMStatePaused
	if _, err := fcClient.PatchVM(ctx, &fcmodels.VM{State: &pausedState}); err != nil {
		return nil, fmt.Errorf("pausing microvm: %w", err)
	}

	// Always resume the VM, even if the snapshot fails, so it is never left paused.
	defer func() {
		resumedState := fcmodels.VMStateResumed
		if _, resumeErr := fcClient.PatchVM(ctx, &fcmodels.VM{State: &resumedState}); resumeErr != nil {
			retErr = errors.Join(retErr, fmt.Errorf("resuming microvm: %w", resumeErr))
		}
	}()

	snapshotType := "Full"
	if _, err := fcClient.CreateSnapshot(ctx, &fcmodels.SnapshotCreateParams{
		SnapshotPath: &statePath,
		MemFilePath:  &memPath,
		SnapshotType: snapshotType,
	}); err != nil {
		return nil, fmt.Errorf("creating snapshot: %w", err)
	}

	logger.Info("snapshot taken")

	return &ports.SnapshotResult{
		Directory: scratch,
		Artifacts: []ports.SnapshotArtifact{
			{Kind: ports.SnapshotState, Path: statePath},
			{Kind: ports.SnapshotMemory, Path: memPath},
		},
	}, nil
}

// isRunning reports whether the firecracker process for the vm is alive.
func (p *fcProvider) isRunning(vmState State) (bool, error) {
	exists, err := afero.Exists(p.fs, vmState.PIDPath())
	if err != nil {
		return false, fmt.Errorf("checking pid file exists: %w", err)
	}

	if !exists {
		return false, nil
	}

	pid, err := vmState.PID()
	if err != nil {
		return false, fmt.Errorf("getting pid from file: %w", err)
	}

	processExists, err := process.Exists(pid)
	if err != nil {
		return false, fmt.Errorf("checking if firecracker process is running: %w", err)
	}

	return processExists, nil
}
