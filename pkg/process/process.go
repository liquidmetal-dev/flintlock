package process

import (
	"fmt"
	"os"
	"syscall"
)

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
