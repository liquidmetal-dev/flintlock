package overlay

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// lockPoll is the time between two attempts to take a held lock.
const lockPoll = 50 * time.Millisecond

// lockFile takes an exclusive flock on the file at path, creating it when
// needed, and returns the function that releases it. It waits until the
// lock is free or ctx ends. It is a file lock and not a mutex because the
// daemon initialises more than one set of ports, and the kernel drops the
// lock when the process dies.
func lockFile(ctx context.Context, path string) (func() error, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, lockPerm)
	if err != nil {
		return nil, fmt.Errorf("opening lock %s: %w", path, err)
	}

	for {
		err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			break
		}

		if !errors.Is(err, unix.EWOULDBLOCK) {
			_ = f.Close()

			return nil, fmt.Errorf("locking %s: %w", path, err)
		}

		select {
		case <-ctx.Done():
			_ = f.Close()

			return nil, fmt.Errorf("waiting for lock %s: %w", path, ctx.Err())
		case <-time.After(lockPoll):
		}
	}

	return func() error {
		if err := unix.Flock(int(f.Fd()), unix.LOCK_UN); err != nil {
			_ = f.Close()

			return fmt.Errorf("unlocking %s: %w", path, err)
		}

		return f.Close()
	}, nil
}
