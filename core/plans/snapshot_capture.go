package plans

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/liquidmetal-dev/flintlock/core/models"
	portsctx "github.com/liquidmetal-dev/flintlock/core/ports/context"
	"github.com/liquidmetal-dev/flintlock/core/steps/snapshot"
	"github.com/liquidmetal-dev/flintlock/pkg/planner"
)

// SnapshotCapturePlanName is the name of the snapshot capture plan.
const SnapshotCapturePlanName = "snapshot_capture"

var errCaptureServicesMissing = errors.New(
	"a snapshot capture needs the microvm and snapshot stores and the volume, image and host info services",
)

// SnapshotCapturePlanInput is the input of the snapshot capture plan.
type SnapshotCapturePlanInput struct {
	// Snapshot is the record of the snapshot to capture.
	Snapshot *models.Snapshot
	// VM is the source microvm.
	VM *models.MicroVM
	// PauseBudget is the longest time that the VMM snapshot and the disk
	// capture can keep the source paused.
	PauseBudget time.Duration
	// Locks are the microvm locks. The plan holds the lock of the source
	// while the source is paused.
	Locks snapshot.Locker
	// FlintlockVersion is the version of flintlock written to the descriptor.
	FlintlockVersion string
}

// SnapshotCapturePlan returns the plan that captures a snapshot of a running
// microvm: lock, check, pause, VMM snapshot, disk capture, resume and unlock,
// descriptor.
func SnapshotCapturePlan(input *SnapshotCapturePlanInput) planner.Plan {
	return &snapshotCapturePlan{input: input}
}

// RunSnapshotCapture runs the snapshot capture plan. When a step fails, it
// resumes the source if it is paused and removes every file of the snapshot
// but its record. The context must hold the ports.
func RunSnapshotCapture(ctx context.Context, input *SnapshotCapturePlanInput, executionID string) error {
	plan := &snapshotCapturePlan{input: input}

	if _, err := planner.NewActuator().Execute(ctx, plan, executionID); err != nil {
		if plan.capture == nil {
			return fmt.Errorf("capturing snapshot %s: %w", input.Snapshot.UID, err)
		}

		return fmt.Errorf("capturing snapshot %s: %w", input.Snapshot.UID, plan.capture.Abort(ctx, err))
	}

	return nil
}

type snapshotCapturePlan struct {
	input   *SnapshotCapturePlanInput
	capture *snapshot.Capture
}

func (p *snapshotCapturePlan) Name() string {
	return SnapshotCapturePlanName
}

// Create returns the steps of the capture the first time, and none after:
// the actuator calls it until it returns no step, and a capture runs once.
func (p *snapshotCapturePlan) Create(ctx context.Context) ([]planner.Procedure, error) {
	if p.capture != nil {
		return nil, nil
	}

	ports, ok := portsctx.GetPorts(ctx)
	if !ok {
		return nil, portsctx.ErrPortsMissing
	}

	vm := p.input.VM

	provider, ok := ports.MicrovmProviders[vm.Spec.Provider]
	if !ok {
		return nil, fmt.Errorf("microvm provider %s isn't available", vm.Spec.Provider)
	}

	if ports.Repo == nil || ports.VolumeService == nil || ports.ImageService == nil ||
		ports.HostInfoService == nil || ports.SnapshotRepo == nil {
		return nil, errCaptureServicesMissing
	}

	p.capture = snapshot.NewCapture(snapshot.CaptureInput{
		Snapshot:         p.input.Snapshot,
		VM:               vm,
		Dir:              ports.SnapshotRepo.Dir(p.input.Snapshot.UID),
		PauseBudget:      p.input.PauseBudget,
		FlintlockVersion: p.input.FlintlockVersion,
		Repo:             ports.Repo,
		Provider:         provider,
		Volumes:          ports.VolumeService,
		Images:           ports.ImageService,
		HostInfo:         ports.HostInfoService,
		Snapshots:        ports.SnapshotRepo,
		Locks:            p.input.Locks,
		FS:               ports.FileSystem,
		Clock:            ports.Clock,
	})

	return snapshot.Steps(p.capture), nil
}

// Finalise does nothing: the phase of the snapshot is set by the caller.
func (p *snapshotCapturePlan) Finalise(_ models.MicroVMState) {}
