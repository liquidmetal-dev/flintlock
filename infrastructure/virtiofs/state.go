package virtiofs

import (
	"fmt"

	"github.com/spf13/afero"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/infrastructure/microvm/shared"
)

const (
	pidVirtioFSFileName    = "virtiofs.pid"
	stdErrVirtioFSFileName = "virtiofs.stderr"
	stdOutVirtioFSFileName = "virtiofs.stdout"
	socketVirtiofsFileName = "virtiofs.sock"
)

type State interface {
	Root() string
	VirtioPID() (int, error)
	VirtioFSPIDPath() string
	SetVirtioFSPid(pid int) error

	VirtioFSPath() string
	ResolveVirtioFSPath() string
	VirtioFSStdoutPath() string
	VirtioFSStderrPath() string
}

func NewState(vmid models.VMID, stateDir, socketDir string, fs afero.Fs) State {
	return &fsState{
		stateRoot:  fmt.Sprintf("%s/%s", stateDir, vmid.String()),
		socketRoot: models.SocketRoot(socketDir, vmid),
		fs:         fs,
	}
}

type fsState struct {
	stateRoot  string
	socketRoot string
	fs         afero.Fs
}

func (s *fsState) Root() string {
	return s.stateRoot
}

// VirtioFSPath is the socket path for a new virtiofsd process to bind to.
func (s *fsState) VirtioFSPath() string {
	return fmt.Sprintf("%s/%s", s.socketRoot, socketVirtiofsFileName)
}

// ResolveVirtioFSPath is the socket path of an already running virtiofsd process. virtiofsd isn't
// restarted when the VMM is re-created, so it may still be on the legacy path.
func (s *fsState) ResolveVirtioFSPath() string {
	return shared.ResolveSocketPath(s.fs, s.VirtioFSPath(), s.legacyVirtioFSPath())
}

// legacyVirtioFSPath is where the socket lived before sockets moved out of the state dir.
// Legacy fallback for #1226, to be removed in the next minor release.
func (s *fsState) legacyVirtioFSPath() string {
	return fmt.Sprintf("%s/%s", s.stateRoot, socketVirtiofsFileName)
}

func (s *fsState) VirtioFSStdoutPath() string {
	return fmt.Sprintf("%s/%s", s.stateRoot, stdOutVirtioFSFileName)
}

func (s *fsState) VirtioFSStderrPath() string {
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
