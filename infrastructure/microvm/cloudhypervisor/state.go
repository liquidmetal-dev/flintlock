package cloudhypervisor

import (
	"fmt"

	"github.com/spf13/afero"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/infrastructure/microvm/shared"
)

const (
	pidFileName       = "cloudhypervisor.pid"
	logFileName       = "cloudhypervisor.log"
	stdOutFileName    = "cloudhypervisor.stdout"
	stdErrFileName    = "cloudhypervisor.stderr"
	stdErrVirtioFSFileName    = "virtiofs.stderr"
	stdOutVirtioFSFileName    = "virtiofs.stdout"
	socketVirtiofsFileName    = "virtiofs.sock"
	socketFileName    = "cloudhypervisor.sock"
	cloudInitFileName = "cloud-init.img"
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

	VirtioFSPath() string
	VirtioFSStdoutPath() string
	VirtioFSStderrPath() string

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
	return fmt.Sprintf("%s/%s", s.stateRoot, pidFileName)
}

func (s *fsState) PID() (int, error) {
	return shared.PIDReadFromFile(s.PIDPath(), s.fs)
}

func (s *fsState) LogPath() string {
	return fmt.Sprintf("%s/%s", s.stateRoot, logFileName)
}

func (s *fsState) StdoutPath() string {
	return fmt.Sprintf("%s/%s", s.stateRoot, stdOutFileName)
}

func (s *fsState) StderrPath() string {
	return fmt.Sprintf("%s/%s", s.stateRoot, stdErrFileName)
}

func (s *fsState) SockPath() string {
	return fmt.Sprintf("%s/%s", s.stateRoot, socketFileName)
}

func (s *fsState) CloudInitImage() string {
	return fmt.Sprintf("%s/%s", s.stateRoot, cloudInitFileName)
}

func (s *fsState) SetPid(pid int) error {
	return shared.PIDWriteToFile(pid, s.PIDPath(), s.fs)
}

func (s *fsState) VirtioFSPath() string {
	return fmt.Sprintf("%s/%s", s.stateRoot, socketVirtiofsFileName)
}

func (s *fsState)  VirtioFSStdoutPath() string {
	return fmt.Sprintf("%s/%s", s.stateRoot, stdOutVirtioFSFileName)
}

func (s *fsState)  VirtioFSStderrPath() string {
	return fmt.Sprintf("%s/%s", s.stateRoot, stdErrVirtioFSFileName)
}