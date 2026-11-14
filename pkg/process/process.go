package process

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// errWaitProcessNotFound is returned when wait() does not find the process.
const errWaitProcessNotFound = "no child processes"

// DetachedStart will start a subprocess in detached mode.
func DetachedStart(cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid:    uint32(os.Getuid()),
			Gid:    uint32(os.Getgid()),
			Groups: []uint32{},
		},
		Setsid: true,
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start process detached: %w", err)
	}

	go func() {
		_, _ = cmd.Process.Wait()
		_ = cmd.Process.Release()
	}()

	return nil
}

// Exists will check if a process exists with the given pid.
func Exists(pid int) (bool, error) {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, fmt.Errorf("finding process with pid %d: %w", pid, err)
	}

	err = proc.Signal(syscall.Signal(0))
	exists := err == nil

	return exists, nil
}

// SendSignal sends a 'sig' signal to 'pid' process.
func SendSignal(pid int, sig os.Signal) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("unable to find process (%d): %w", pid, err)
	}

	if err := proc.Signal(sig); err != nil {
		return fmt.Errorf(
			"unable to send signal to process (%s): %w",
			sig.String(),
			err,
		)
	}

	return nil
}

// WaitWithContext will wait for a process to exit.
// It will return the exit code of the process.
func WaitWithContext(ctx context.Context, pid int) error {
	// on Unix system os.FindProcess() is always successful
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("unable to find process (%d): %w", pid, err)
	}

	exited := make(chan error)

	go func() {
		_, err := proc.Wait()
		if err != nil {
			exited <- fmt.Errorf("wait for process (%d): %w", pid, err)
		}
		exited <- nil
	}()

	select {
	case err := <-exited:
		return err
	case <-ctx.Done():
		// If the context is canceled, we need to kill the process.
		// This is a best effort attempt.
		// It only kills the child process, not its children.
		if err := proc.Kill(); err != nil {
			return fmt.Errorf("unable to kill process (%d): %w", pid, err)
		}

		// release the pid
		if _, err := proc.Wait(); err != nil {
			// ignore error if process is already cleaned up
			if !strings.Contains(err.Error(), errWaitProcessNotFound) {
				return fmt.Errorf("unable to release process (%d): %w", pid, err)
			}
		}

		return nil
	}
}
