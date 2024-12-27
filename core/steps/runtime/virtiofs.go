package runtime

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"

	cerrs "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/infrastructure/virtiofs"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
	"github.com/liquidmetal-dev/flintlock/pkg/planner"
	"github.com/liquidmetal-dev/flintlock/pkg/process"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
)

func NewVirtioFSMount(vmid *models.VMID,
	volume *models.Volume,
	status *models.VolumeStatus,
	stateDir string,
) planner.Procedure {
	return &volumeVirtioFSMount{
		vmid:   vmid,
		volume: volume,
		status: status,
		stateDir: stateDir,
	}
}

type volumeVirtioFSMount struct {
	vmid     *models.VMID
	volume   *models.Volume
	status   *models.VolumeStatus
	stateDir string 
}

// Name is the name of the procedure/operation.
func (s *volumeVirtioFSMount) Name() string {
	return "runtime_virtiofs"
}

func (s *volumeVirtioFSMount) ShouldDo(ctx context.Context) (bool, error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step": s.Name(),
		"id":   s.volume.ID,
	})
	logger.Debug("checking if procedure should be run")

	if s.status == nil || s.status.Mount.Source == "" {
		return true, nil
	}
	//TODO: MAKE THIS A VALID CHECK IF IT's MOUNTED
	mounted := false

	return !mounted,nil
}

// Do will perform the operation/procedure.
func (s *volumeVirtioFSMount) Do(ctx context.Context) ([]planner.Procedure, error) {
	if s.status == nil {
		return nil, cerrs.ErrMissingStatusInfo
	}

	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step": s.Name(),
		"id":   s.volume.ID,
	})
	virtiofsState := virtiofs.NewVirtioFSState(*s.vmid, s.stateDir)
	err := startVirtioFS(ctx, virtiofsState, s)
	logger.Debug("running step for virtiofs volume: ", virtiofsState.VirtioFSPath())
	if err != nil {
		return nil,fmt.Errorf("starting cloudhypervisor process: %w", err)
	}
	return nil,nil
}

func (s *volumeVirtioFSMount) Verify(_ context.Context) error {
	return nil
}


func startVirtioFS(ctx context.Context, virtiofsState virtiofs.VirtioFSState, s *volumeVirtioFSMount) (error) {
	fs := afero.NewOsFs()
	options := fmt.Sprintf("source=%s,cache=none,sandbox=chroot,announce_submounts,allow_direct_io", s.volume.Source.VirtioFS.Path)
    cmdVirtioFS := exec.Command("/usr/libexec/virtiofsd",
        "--socket-path="+virtiofsState.VirtioFSPath(),
		"--thread-pool-size=32",
        "-o", options)
	stdOutFileVirtioFS, err := fs.OpenFile(virtiofsState.VirtioFSStdoutPath(), os.O_WRONLY|os.O_CREATE|os.O_APPEND, defaults.DataFilePerm)
	if err != nil {
		return fmt.Errorf("opening stdout file %s: %w", virtiofsState.VirtioFSStdoutPath(), err)
	}
	
	stdErrFileVirtioFS, err := fs.OpenFile(virtiofsState.VirtioFSStderrPath(), os.O_WRONLY|os.O_CREATE|os.O_APPEND, defaults.DataFilePerm)
	if err != nil {
		return fmt.Errorf("opening sterr file %s: %w", virtiofsState.VirtioFSStderrPath(), err)
	}

	cmdVirtioFS.Stderr = stdErrFileVirtioFS
	cmdVirtioFS.Stdout = stdOutFileVirtioFS
	cmdVirtioFS.Stdin = &bytes.Buffer{}

	var startErr error
	process.DetachedStart(cmdVirtioFS)

	if startErr != nil {
		return fmt.Errorf("starting virtiofsd process: %w", err)
	}
	return nil
}