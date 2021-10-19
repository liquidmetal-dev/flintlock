package process

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

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
