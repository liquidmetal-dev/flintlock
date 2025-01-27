package virtiofs

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/internal/config"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
	"github.com/liquidmetal-dev/flintlock/pkg/process"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
)

// New will create a new instance of the VirtioFS.
func New(cfg *config.Config,
	fs afero.Fs,
) ports.VirtioFSService {
	return &vFSService{
		config: cfg,
		fs:     fs,
	}
}

type vFSService struct {
	config *config.Config
	fs     afero.Fs
}

// Create will start and create a virtiofsd process.
func (s *vFSService) Create(ctx context.Context, 
	vmid *models.VMID, 
	input ports.VirtioFSCreateInput) (*models.Mount, error) {
	state := NewState(*vmid, s.config.StateRootDir+"/vm", s.fs)
	if err := s.ensureState(state); err != nil {
		return nil, fmt.Errorf("ensuring state dir: %w", err)
	}
	procVFS, err := s.startVirtioFS(ctx, input, state)
	if err != nil {
		return nil, fmt.Errorf("starting virtiofs process: %w", err)
	}
	if err = state.SetVirtioFSPid(procVFS.Pid); err != nil {
		return nil, fmt.Errorf("saving pid %d to file: %w", procVFS.Pid, err)
	}
	mount := models.Mount{
		Source: state.VirtioFSPath(),
		Type:   "hostpath",
	}

	return &mount, nil
}

func (s *vFSService) Delete(ctx context.Context, vmid *models.VMID) error {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"service": "virtiofs_delete",
		"vmid":    vmid.String(),
	})
	state := NewState(*vmid, s.config.StateRootDir+"/vm", s.fs)
	pid, pidErr := state.VirtioPID()
	if pidErr != nil {
		fmt.Println(fmt.Errorf("unable to get PID: %w", pidErr))
		
		return nil
	}
	processExists, err := process.Exists(pid)
	if err != nil {
		return fmt.Errorf("checking if virtiofsd process is running: %w", err)
	}
	if !processExists {
		return nil
	}
	logger.Debugf("sending SIGTERM to %d", pid)

	if sigErr := process.SendSignal(pid, syscall.SIGTERM); sigErr != nil {
		return fmt.Errorf("failed to terminate with SIGHUP: %w", sigErr)
	}

	ctxTimeout, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()

	// Make sure the virtiofsd is stopped.
	if err := process.WaitWithContext(ctxTimeout, pid); err != nil {
		return fmt.Errorf("failed to wait for pid %d: %w", pid, err)
	}

	return nil
}

func (s *vFSService) HasVirtioFSDProcess(_ context.Context, vmid *models.VMID) (bool, error) {
	state := NewState(*vmid, s.config.StateRootDir+"/vm", s.fs)
	pid, pidErr := state.VirtioPID()
	if pidErr != nil {
		return false, pidErr
	}
	processExists, err := process.Exists(pid)
	if err != nil {
		return false, err
	}

	return processExists, nil
}

func (s *vFSService) startVirtioFS(_ context.Context,
	input ports.VirtioFSCreateInput,
	state State,
) (*os.Process, error) {
	options := fmt.Sprintf("source=%s,cache=none,sandbox=chroot,announce_submounts,allow_direct_io", input.Path)
	cmdVirtioFS := exec.Command(s.config.VirtioFSBin,
		"--socket-path="+state.VirtioFSPath(),
		"--thread-pool-size=32",
		"-o", options)
	stdOutFileVirtioFS, err := s.fs.OpenFile(state.VirtioFSStdoutPath(), 
		os.O_WRONLY|os.O_CREATE|os.O_APPEND, 
		defaults.DataFilePerm)
	if err != nil {
		return nil, fmt.Errorf("opening stdout file %s: %w", state.VirtioFSStdoutPath(), err)
	}

	stdErrFileVirtioFS, err := s.fs.OpenFile(state.VirtioFSStderrPath(), 
		os.O_WRONLY|os.O_CREATE|os.O_APPEND, 
		defaults.DataFilePerm)
	if err != nil {
		return nil, fmt.Errorf("opening sterr file %s: %w", state.VirtioFSStderrPath(), err)
	}

	cmdVirtioFS.Stderr = stdErrFileVirtioFS
	cmdVirtioFS.Stdout = stdOutFileVirtioFS
	cmdVirtioFS.Stdin = &bytes.Buffer{}

	var startErr error
	process.DetachedStart(cmdVirtioFS)

	if startErr != nil {
		return nil, fmt.Errorf("starting virtiofsd process: %w", err)
	}

	return cmdVirtioFS.Process, nil
}

func (s *vFSService) ensureState(state State) error {
	exists, err := afero.DirExists(s.fs, state.Root())
	if err != nil {
		return fmt.Errorf("checking if state dir %s exists: %w", state.Root(), err)
	}

	if !exists {
		if err = s.fs.MkdirAll(state.Root(), defaults.DataDirPerm); err != nil {
			return fmt.Errorf("creating state directory %s: %w", state.Root(), err)
		}
	}

	return nil
}
