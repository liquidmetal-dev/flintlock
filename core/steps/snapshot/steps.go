package snapshot

import (
	"context"
	"fmt"

	coreerrs "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/pkg/planner"
)

const (
	CheckStepName       = "snapshot_check"
	PauseStepName       = "snapshot_pause"
	VMMSnapshotStepName = "snapshot_vmm"
	DiskCaptureStepName = "snapshot_disk_capture"
	ResumeStepName      = "snapshot_resume"
	DescriptorStepName  = "snapshot_descriptor"
)

// Steps returns the steps of the capture, in the order they run.
func Steps(c *Capture) []planner.Procedure {
	return []planner.Procedure{
		&step{name: CheckStepName, do: c.check},
		&step{name: PauseStepName, do: c.lockAndPause},
		&step{name: VMMSnapshotStepName, do: c.vmmSnapshot},
		&step{name: DiskCaptureStepName, do: c.diskCapture},
		&step{name: ResumeStepName, do: c.resumeAndUnlock},
		&step{name: DescriptorStepName, do: c.writeFiles},
	}
}

// step is one step of a capture. Every step runs once: the plan returns the
// steps of a capture once.
type step struct {
	name string
	do   func(ctx context.Context) error
}

func (s *step) Name() string {
	return s.name
}

func (s *step) ShouldDo(_ context.Context) (bool, error) {
	return true, nil
}

func (s *step) Do(ctx context.Context) ([]planner.Procedure, error) {
	return nil, s.do(ctx)
}

func (s *step) Verify(_ context.Context) error {
	return nil
}

func (c *Capture) check(ctx context.Context) error {
	if err := CheckSource(c.in.VM, c.in.Provider); err != nil {
		return err
	}

	state, err := c.in.Provider.State(ctx, c.in.VM.ID.String())
	if err != nil {
		return fmt.Errorf("getting the state of microvm %s: %w", c.in.VM.ID, err)
	}

	if state != ports.MicroVMStateRunning {
		return coreerrs.NewFailedPrecondition(
			fmt.Sprintf("the vmm of microvm %s is not running (state: %s)", c.in.VM.ID.UID(), state),
		)
	}

	return nil
}

func (c *Capture) lockAndPause(ctx context.Context) error {
	c.lock()

	return c.pause(ctx)
}

func (c *Capture) vmmSnapshot(ctx context.Context) error {
	return c.withinBudget(ctx, func(ctx context.Context) error {
		if err := c.in.Provider.Snapshot(ctx, c.in.VM, c.in.Dir); err != nil {
			return fmt.Errorf("taking the vmm snapshot of microvm %s: %w", c.in.VM.ID, err)
		}

		return nil
	})
}

func (c *Capture) diskCapture(ctx context.Context) error {
	ref := ports.VolumeRef{VMID: c.in.VM.ID, VolumeID: c.in.VM.Spec.RootVolume.ID}

	return c.withinBudget(ctx, func(ctx context.Context) error {
		volume, err := c.in.Volumes.Capture(ctx, ref, c.in.Dir)
		if err != nil {
			return fmt.Errorf("capturing the root volume of microvm %s: %w", c.in.VM.ID, err)
		}

		c.volume = volume

		return nil
	})
}

func (c *Capture) resumeAndUnlock(ctx context.Context) error {
	if err := c.resume(ctx); err != nil {
		return err
	}

	c.unlock()

	return nil
}
