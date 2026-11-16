package snapshot

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
)

// Locker locks a microvm by its uid.
type Locker interface {
	// Lock takes the lock of the microvm, and waits until it is free.
	Lock(uid string)
	// Unlock releases the lock of the microvm.
	Unlock(uid string)
}

// PauseBudgetExceededError tells that the source microvm was paused for longer
// than the pause budget allows.
type PauseBudgetExceededError struct {
	Budget time.Duration
}

// Error returns the error message.
func (e *PauseBudgetExceededError) Error() string {
	return fmt.Sprintf("pause budget of %s exceeded", e.Budget)
}

// CaptureInput is the input of a capture.
type CaptureInput struct {
	// Snapshot is the record of the snapshot.
	Snapshot *models.Snapshot
	// VM is the source microvm.
	VM *models.MicroVM
	// Dir is the directory of the snapshot on the host.
	Dir string
	// PauseBudget is the longest time that the VMM snapshot and the disk
	// capture can keep the source paused.
	PauseBudget time.Duration
	// FlintlockVersion is the version of flintlock written to the descriptor.
	FlintlockVersion string

	Provider  ports.MicroVMService
	Volumes   ports.VolumeService
	Images    ports.ImageService
	HostInfo  ports.HostInfoService
	Snapshots ports.SnapshotRepository
	Locks     Locker
	FS        afero.Fs
	Clock     func() time.Time
}

// Capture is the state that the steps of one capture share. Use NewCapture.
type Capture struct {
	in CaptureInput

	locked        bool
	paused        bool
	pausedAt      time.Time
	pauseDeadline time.Time
	volume        *models.CapturedVolume
}

// NewCapture returns the state of a capture that has not started.
func NewCapture(in CaptureInput) *Capture {
	return &Capture{in: in}
}

// Abort makes the source and the snapshot directory safe after a step failed
// with cause. It resumes the source if it is paused, releases the lock of the
// source, and removes every file of the snapshot but its record. It returns
// cause, with the error of a failed resume or removal.
func (c *Capture) Abort(ctx context.Context, cause error) error {
	// The context may be the reason the step failed, and the source must be
	// resumed regardless.
	ctx = context.WithoutCancel(ctx)
	logger := c.logger(ctx)

	err := cause

	if c.paused {
		if resumeErr := c.resume(ctx); resumeErr != nil {
			logger.WithError(resumeErr).Error("resuming the source microvm after a failed snapshot")

			err = fmt.Errorf("%w; and resuming the source microvm failed: %w", err, resumeErr)
		}
	}

	c.unlock()

	if removeErr := c.in.Snapshots.RemoveFiles(ctx, c.in.Snapshot.UID); removeErr != nil {
		logger.WithError(removeErr).Error("removing the files of a failed snapshot")

		err = fmt.Errorf("%w; and removing the files of the snapshot failed: %w", err, removeErr)
	}

	return err
}

func (c *Capture) lock() {
	c.in.Locks.Lock(c.in.VM.ID.UID())
	c.locked = true
}

func (c *Capture) unlock() {
	if !c.locked {
		return
	}

	c.in.Locks.Unlock(c.in.VM.ID.UID())
	c.locked = false
}

func (c *Capture) pause(ctx context.Context) error {
	if err := c.in.Provider.Pause(ctx, c.in.VM); err != nil {
		return fmt.Errorf("pausing microvm %s: %w", c.in.VM.ID, err)
	}

	c.paused = true
	c.pausedAt = c.in.Clock()
	c.pauseDeadline = time.Now().Add(c.in.PauseBudget)

	return nil
}

func (c *Capture) resume(ctx context.Context) error {
	if err := c.in.Provider.Resume(ctx, c.in.VM); err != nil {
		return fmt.Errorf("resuming microvm %s: %w", c.in.VM.ID, err)
	}

	c.paused = false

	c.logger(ctx).WithField("pause_time", c.in.Clock().Sub(c.pausedAt).String()).Info("resumed the source microvm")

	return nil
}

// withinBudget runs fn with a context that ends when the pause budget is
// spent. It returns a PauseBudgetExceededError when the budget was spent
// before fn returned.
func (c *Capture) withinBudget(ctx context.Context, fn func(ctx context.Context) error) error {
	budgetCtx, cancel := context.WithDeadline(ctx, c.pauseDeadline)
	defer cancel()

	if budgetCtx.Err() == nil {
		if err := fn(budgetCtx); err != nil && !c.budgetSpent(ctx, budgetCtx) {
			return err
		}
	}

	if c.budgetSpent(ctx, budgetCtx) {
		return &PauseBudgetExceededError{Budget: c.in.PauseBudget}
	}

	return nil
}

func (c *Capture) budgetSpent(parent, budgetCtx context.Context) bool {
	return parent.Err() == nil && errors.Is(budgetCtx.Err(), context.DeadlineExceeded)
}

func (c *Capture) logger(ctx context.Context) *logrus.Entry {
	return log.GetLogger(ctx).WithFields(logrus.Fields{
		"snapshot": c.in.Snapshot.UID,
		"vmid":     c.in.VM.ID.String(),
	})
}
