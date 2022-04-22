package cloudhypervisor

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"

	"github.com/spf13/afero"
	"github.com/weaveworks-liquidmetal/flintlock/core/models"
	"github.com/weaveworks-liquidmetal/flintlock/pkg/defaults"
)

type State interface {
	Root() string

	PID() (int, error)
	PIDPath() string
	SetPid(pid int) error

	LogPath() string
	StdoutPath() string
	StderrPath() string
	SockPath() string

	//MetadataDir() string
	CloudInitImage() string
}

func NewState(vmid models.VMID, stateDir string, fs afero.Fs) State {
	return &fsState{
		stateRoot: fmt.Sprintf("%s/%s", stateDir, vmid.String()),
		fs:        fs,
	}
}

type fsState struct {
	stateRoot string
	fs        afero.Fs
}

func (s *fsState) Root() string {
	return s.stateRoot
}

func (s *fsState) PIDPath() string {
	return fmt.Sprintf("%s/cloudhypervisor.pid", s.stateRoot)
}

func (s *fsState) PID() (int, error) {
	return s.pidReadFromFile(s.PIDPath())
}

func (s *fsState) LogPath() string {
	return fmt.Sprintf("%s/cloudhypervisor.log", s.stateRoot)
}

func (s *fsState) StdoutPath() string {
	return fmt.Sprintf("%s/cloudhypervisor.stdout", s.stateRoot)
}

func (s *fsState) StderrPath() string {
	return fmt.Sprintf("%s/cloudhypervisor.stderr", s.stateRoot)
}

func (s *fsState) SockPath() string {
	return fmt.Sprintf("%s/cloudhypervisor.sock", s.stateRoot)
}

// func (s *fsState) MetadataDir() string {
// 	return fmt.Sprintf("%s/metadata", s.stateRoot)
// }

func (s *fsState) CloudInitImage() string {
	return fmt.Sprintf("%s/cloud-init.img", s.stateRoot)
}

func (s *fsState) SetPid(pid int) error {
	return s.pidWriteToFile(pid, s.PIDPath())
}

func (s *fsState) pidReadFromFile(pidFile string) (int, error) {
	file, err := s.fs.Open(pidFile)
	if err != nil {
		return -1, fmt.Errorf("opening pid file %s: %w", pidFile, err)
	}

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return -1, fmt.Errorf("reading pid file %s: %w", pidFile, err)
	}

	pid, err := strconv.Atoi(string(bytes.TrimSpace(data)))
	if err != nil {
		return -1, fmt.Errorf("converting data to int: %w", err)
	}

	return pid, nil
}

func (s *fsState) pidWriteToFile(pid int, pidFile string) error {
	file, err := s.fs.OpenFile(pidFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, defaults.DataFilePerm)
	if err != nil {
		return fmt.Errorf("opening pid file %s: %w", pidFile, err)
	}

	defer file.Close()

	_, err = fmt.Fprintf(file, "%d", pid)
	if err != nil {
		return fmt.Errorf("writing pid %d to file %s: %w", pid, pidFile, err)
	}

	return nil
}
