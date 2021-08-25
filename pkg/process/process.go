package process

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// StartCommandDetached will start the given cmd so its detached from its parent process.
func StartCommandDetached(cmd *exec.Cmd, stdErrFile *os.File, stdOutFile *os.File) (*os.Process, error) {
	groups, err := os.Getgroups()
	if err != nil {
		return nil, fmt.Errorf("get os groups: %w", err)
	}
	groupsConv := []uint32{}
	for _, groupID := range groups {
		groupsConv = append(groupsConv, uint32(groupID))
	}

	files := []*os.File{nil, stdOutFile, stdErrFile}

	attr := &os.ProcAttr{
		Dir:   "./",
		Files: files,
		Sys: &syscall.SysProcAttr{
			Credential: &syscall.Credential{
				Uid:    uint32(os.Getuid()),
				Gid:    uint32(os.Getgid()),
				Groups: groupsConv,
			},
			Setsid: true,
		},
	}

	proc, err := os.StartProcess(cmd.Path, cmd.Args, attr)
	if err != nil {
		return nil, fmt.Errorf("starting process: %w", err)
	}

	return proc, nil
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
