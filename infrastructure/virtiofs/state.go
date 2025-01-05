package virtiofs

import (
	"fmt"

	"github.com/spf13/afero"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/infrastructure/microvm/shared"
)

const (
	pidVirtioFSFileName       = "virtiofs.pid"
	stdErrVirtioFSFileName    = "virtiofs.stderr"
	stdOutVirtioFSFileName    = "virtiofs.stdout"
	socketVirtiofsFileName    = "virtiofs.sock"
)

type State interface {

	Root() string
	VirtioPID() (int, error)
	VirtioFSPIDPath() string
	SetVirtioFSPid(pid int) error
	
	VirtioFSPath() string
	VirtioFSStdoutPath() string
	VirtioFSStderrPath() string

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

func (s *fsState) VirtioFSPath() string {
	return fmt.Sprintf("%s/%s", s.stateRoot, socketVirtiofsFileName)
}

func (s *fsState)  VirtioFSStdoutPath() string {
	return fmt.Sprintf("%s/%s", s.stateRoot, stdOutVirtioFSFileName)
}

func (s *fsState)  VirtioFSStderrPath() string {
	return fmt.Sprintf("%s/%s", s.stateRoot, stdErrVirtioFSFileName)
}

func (s *fsState) VirtioFSPIDPath() string {
	return fmt.Sprintf("%s/%s", s.stateRoot, pidVirtioFSFileName)
}

func (s *fsState) VirtioPID() (int, error) {
	return shared.PIDReadFromFile(s.VirtioFSPIDPath(), s.fs)
}

func (s *fsState) SetVirtioFSPid(pid int) error {
	return shared.PIDWriteToFile(pid, s.VirtioFSPIDPath(), s.fs)
}