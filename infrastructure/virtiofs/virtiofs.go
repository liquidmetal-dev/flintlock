package virtiofs

import (
	"fmt"
	
	"github.com/liquidmetal-dev/flintlock/core/models"
)


type VirtioFSState interface {

	// VirtioPID() (int, error)
	VirtioFSPIDPath() string
	// SetVirtioFSPid(pid int) error
	
	VirtioFSPath() string
	VirtioFSStdoutPath() string
	VirtioFSStderrPath() string
}

func NewVirtioFSState(vmid models.VMID, stateDir string) VirtioFSState {
	return &vFSState{
		stateRoot: fmt.Sprintf("%s/%s", stateDir, vmid.String()),
	}
}

type vFSState struct {
	stateRoot string
}

func (s *vFSState) VirtioFSPath() string {
	return fmt.Sprintf("%s/%s", s.stateRoot, "/virtiofs.sock")
}

func (s *vFSState)  VirtioFSStdoutPath() string {
	return fmt.Sprintf("%s/%s", s.stateRoot, "/virtiofs.stdout")
}

func (s *vFSState)  VirtioFSStderrPath() string {
	return fmt.Sprintf("%s/%s", s.stateRoot, "/virtiofs.pid")
}

func (s *vFSState) VirtioFSPIDPath() string {
	return fmt.Sprintf("%s/%s", s.stateRoot, "/virtiofs.pid")
}