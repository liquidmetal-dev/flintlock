package cloudhypervisor

import (
	"fmt"

	"github.com/spf13/afero"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/infrastructure/microvm/shared"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
)

const (
	pidFileName       = "cloudhypervisor.pid"
	logFileName       = "cloudhypervisor.log"
	stdOutFileName    = "cloudhypervisor.stdout"
	stdErrFileName    = "cloudhypervisor.stderr"
	socketFileName    = "cloudhypervisor.sock"
	cloudInitFileName = "cloud-init.img"
)

type State interface {
	Root() string
	SocketRoot() string

	PID() (int, error)
	PIDPath() string
	SetPid(pid int) error

	LogPath() string
	StdoutPath() string
	StderrPath() string
	SockPath() string
	ResolveSockPath() string
	VSockPath() string
	legacySockPath() string
	legacyVSockPath() string

	CloudInitImage() string
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

func (s *fsState) SocketRoot() string {
	return s.socketRoot
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

// SockPath is the API socket path for a new cloud-hypervisor process to bind to.
func (s *fsState) SockPath() string {
	return fmt.Sprintf("%s/%s", s.socketRoot, socketFileName)
}

// ResolveSockPath is the API socket path of an already running cloud-hypervisor process.
func (s *fsState) ResolveSockPath() string {
	return shared.ResolveSocketPath(s.fs, s.SockPath(), s.legacySockPath())
}

func (s *fsState) VSockPath() string {
	return fmt.Sprintf("%s/%s", s.socketRoot, defaults.GuestAgentVsockName)
}

// legacySockPath is where the API socket lived before sockets moved out of the state dir.
// Legacy fallback for #1226, to be removed in the next minor release.
func (s *fsState) legacySockPath() string {
	return fmt.Sprintf("%s/%s", s.stateRoot, socketFileName)
}

// legacyVSockPath is where the vsock socket lived before sockets moved out of the state dir.
// Legacy fallback for #1226, to be removed in the next minor release.
func (s *fsState) legacyVSockPath() string {
	return fmt.Sprintf("%s/%s", s.stateRoot, defaults.GuestAgentVsockName)
}

func (s *fsState) CloudInitImage() string {
	return fmt.Sprintf("%s/%s", s.stateRoot, cloudInitFileName)
}

func (s *fsState) SetPid(pid int) error {
	return shared.PIDWriteToFile(pid, s.PIDPath(), s.fs)
}
