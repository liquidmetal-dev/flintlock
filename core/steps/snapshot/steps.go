package snapshot

import (
	"context"
	"fmt"

	coreerrs "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/pkg/planner"
)

const (
	LockStepName        = "snapshot_lock"
	CheckStepName       = "snapshot_check"
	PauseStepName       = "snapshot_pause"
	VMMSnapshotStepName = "snapshot_vmm"
	DiskCaptureStepName = "snapshot_disk_capture"
	ResumeStepName      = "snapshot_resume"
	DescriptorStepName  = "snapshot_descriptor"
)

// Steps returns the steps of the capture, in the order they run. The lock of
// the source is taken before the check, so the source cannot change between
// the check and the pause.
func Steps(c *Capture) []planner.Procedure {
	return []planner.Procedure{
		&step{name: LockStepName, do: c.lock},
		&step{name: CheckStepName, do: c.check},
		&step{name: PauseStepName, do: c.pause},
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

// check reads the source again and checks that it can be snapshotted. The
// caller holds the lock of the source.
func (c *Capture) check(ctx context.Context) error {
	uid := c.vm.ID.UID()

	vm, err := c.in.Repo.Get(ctx, ports.RepositoryGetOptions{UID: uid})
	if err != nil {
		return fmt.Errorf("getting source microvm %s: %w", uid, err)
	}

	if vm == nil {
		return coreerrs.NewSpecNotFound("", "", "", uid)
	}

	c.vm = vm

	if err := CheckSource(c.vm, c.in.Provider); err != nil {
		return err
	}

	state, err := c.in.Provider.State(ctx, c.vm.ID.String())
	if err != nil {
		return fmt.Errorf("getting the state of microvm %s: %w", c.vm.ID, err)
	}

	if state != ports.MicroVMStateRunning {
		return coreerrs.NewFailedPrecondition(
			fmt.Sprintf("the vmm of microvm %s is not running (state: %s)", c.vm.ID.UID(), state),
		)
	}

	return nil
}

func (c *Capture) vmmSnapshot(ctx context.Context) error {
	return c.withinBudget(ctx, func(ctx context.Context) error {
		if err := c.in.Provider.Snapshot(ctx, c.vm, c.in.Dir); err != nil {
			return fmt.Errorf("taking the vmm snapshot of microvm %s: %w", c.vm.ID, err)
		}

		return nil
	})
}

func (c *Capture) diskCapture(ctx context.Context) error {
	ref := ports.VolumeRef{VMID: c.vm.ID, VolumeID: c.vm.Spec.RootVolume.ID}

	return c.withinBudget(ctx, func(ctx context.Context) error {
		volume, err := c.in.Volumes.Capture(ctx, ref, c.in.Dir)
		if err != nil {
			return fmt.Errorf("capturing the root volume of microvm %s: %w", c.vm.ID, err)
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
