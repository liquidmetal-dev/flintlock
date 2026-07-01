package cloudhypervisor

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	cerrs "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/pkg/cloudhypervisor"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
)

const snapshotResumeTimeout = 10 * time.Second

// Snapshot pauses a running microvm, captures a snapshot to disk and resumes it.
func (p *provider) Snapshot(
	ctx context.Context,
	input ports.SnapshotInput,
) (result *ports.SnapshotResult, retErr error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"service": "cloudhypervisor_microvm",
		"vmid":    input.VMID.String(),
	})
	logger.Info("snapshotting microvm")

	vmState := NewState(input.VMID, p.config.StateRoot, p.fs)
	chClient := cloudhypervisor.New(vmState.SockPath())

	// Pre-flight: only a running VM can be snapshotted.
	info, err := chClient.Info(ctx)
	if err != nil {
		return nil, fmt.Errorf("querying cloud-hypervisor for info: %w", err)
	}

	if info.State != cloudhypervisor.VMStateRunning {
		return nil, fmt.Errorf("microvm is not running (state: %s): %w", info.State, cerrs.ErrNotRunning)
	}

	scratch := vmState.SnapshotPath()
	if err := p.fs.MkdirAll(scratch, defaults.DataDirPerm); err != nil {
		return nil, fmt.Errorf("creating snapshot scratch dir %s: %w", scratch, err)
	}

	if err := chClient.Pause(ctx); err != nil {
		return nil, fmt.Errorf("pausing microvm: %w", err)
	}

	// Always resume the VM, even if the snapshot fails, so it is never left paused.
	defer func() {
		resumeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), snapshotResumeTimeout)
		defer cancel()

		if resumeErr := chClient.Resume(resumeCtx); resumeErr != nil {
			retErr = errors.Join(retErr, fmt.Errorf("resuming microvm: %w", resumeErr))
		}
	}()

	destURL := "file://" + scratch
	if err := chClient.Snapshot(ctx, &cloudhypervisor.VMSnapshotConfig{DestinationURL: &destURL}); err != nil {
		return nil, fmt.Errorf("creating snapshot: %w", err)
	}

	logger.Info("snapshot taken")

	return &ports.SnapshotResult{
		Directory: scratch,
		Artifacts: []ports.SnapshotArtifact{
			{Kind: ports.SnapshotState, Path: scratch + "/state.json"},
			{Kind: ports.SnapshotMemory, Path: scratch + "/memory-ranges"},
			{Kind: ports.SnapshotConfig, Path: scratch + "/config.json"},
		},
	}, nil
}
