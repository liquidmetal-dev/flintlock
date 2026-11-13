package firecracker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/firecracker-microvm/firecracker-go-sdk"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/infrastructure/microvm/shared"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
	"github.com/liquidmetal-dev/flintlock/pkg/process"
)

// socketDirPerm is the mode of the socket directory of a microvm.
const socketDirPerm = 0o700

var errSymlinkNotSupported = errors.New("file system does not support symlinks")

// Create will create a new microvm.
func (p *fcProvider) Create(ctx context.Context, vm *models.MicroVM) error {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"service": "firecracker_microvm",
		"vmid":    vm.ID.String(),
	})
	logger.Debugf("creating microvm")

	vmState := NewState(vm.ID, p.config.StateRoot, p.config.SocketDir, p.fs)

	if err := p.ensureState(vmState); err != nil {
		return fmt.Errorf("ensuring state dir: %w", err)
	}

	config, err := CreateConfig(WithMicroVM(vm), WithState(vmState), WithVsock(vm, vmState))
	if err != nil {
		return fmt.Errorf("creating firecracker config: %w", err)
	}

	if vm.Spec.AllowGuestAgent {
		vm.Status.VSockPath = vmState.VSockPath()
	} else {
		vm.Status.VSockPath = ""
	}

	if err = vmState.SetConfig(config); err != nil {
		return fmt.Errorf("saving firecracker config: %w", err)
	}

	meta := &Metadata{
		Latest: vm.Spec.Metadata,
	}

	if err = vmState.SetMetadata(meta); err != nil {
		return fmt.Errorf("saving firecracker metadata: %w", err)
	}

	proc, err := p.startFirecracker(p.command(vm, vmState), vmState, p.config.RunDetached)
	if err != nil {
		return fmt.Errorf("starting firecracker process: %w", err)
	}

	if err = vmState.SetPid(proc.Pid); err != nil {
		return fmt.Errorf("saving pid %d to file: %w", proc.Pid, err)
	}

	return nil
}

// command returns the command that starts firecracker for a microvm. The
// working directory of the process is the state directory of the microvm, so
// that the relative drive paths of an overlay root resolve to its own disks.
func (p *fcProvider) command(vm *models.MicroVM, vmState State) *exec.Cmd {
	args := []string{"--id", vm.ID.UID(), "--boot-timer", "--api-sock", vmState.APISockPath()}
	args = append(args, "--config-file", vmState.ConfigPath())
	args = append(args, "--metadata", vmState.MetadataPath())

	cmd := firecracker.VMCommandBuilder{}.
		WithBin(p.config.FirecrackerBin).
		WithArgs(args).
		Build(context.TODO()) //nolint: contextcheck // Intentional.

	cmd.Dir = vmState.Root()

	return cmd
}

func (p *fcProvider) startFirecracker(cmd *exec.Cmd, vmState State, detached bool) (*os.Process, error) {
	stdOutFile, err := p.fs.OpenFile(vmState.StdoutPath(), os.O_WRONLY|os.O_CREATE|os.O_APPEND, defaults.DataFilePerm)
	if err != nil {
		return nil, fmt.Errorf("opening stdout file %s: %w", vmState.StdoutPath(), err)
	}

	stdErrFile, err := p.fs.OpenFile(vmState.StderrPath(), os.O_WRONLY|os.O_CREATE|os.O_APPEND, defaults.DataFilePerm)
	if err != nil {
		return nil, fmt.Errorf("opening sterr file %s: %w", vmState.StderrPath(), err)
	}

	cmd.Stderr = stdErrFile
	cmd.Stdout = stdOutFile
	cmd.Stdin = &bytes.Buffer{}

	var startErr error

	if detached {
		startErr = process.DetachedStart(cmd)
	} else {
		startErr = cmd.Start()
	}

	if startErr != nil {
		return nil, fmt.Errorf("starting firecracker process: %w", startErr)
	}

	return cmd.Process, nil
}

func (p *fcProvider) ensureState(vmState State) error {
	exists, err := afero.DirExists(p.fs, vmState.Root())
	if err != nil {
		return fmt.Errorf("checking if state dir %s exists: %w", vmState.Root(), err)
	}

	if !exists {
		if err = p.fs.MkdirAll(vmState.Root(), defaults.DataDirPerm); err != nil {
			return fmt.Errorf("creating state directory %s: %w", vmState.Root(), err)
		}
	}

	if err := p.ensureSocketLink(vmState); err != nil {
		return err
	}

	// Remove any stale guest-agent vsock socket so the VMM can bind on (re)create. The VMM isn't
	// running, so a socket left in the state dir by an older flintlock can go too.
	if err := shared.RemoveStaleSockets(p.fs, vmState.VSockPath(), vmState.legacyVSockPath()); err != nil {
		return fmt.Errorf("removing stale vsock socket: %w", err)
	}

	if err := shared.RemoveStaleSockets(p.fs, vmState.APISockPath()); err != nil {
		return fmt.Errorf("removing stale api socket: %w", err)
	}

	logFile, err := p.fs.OpenFile(vmState.LogPath(), os.O_WRONLY|os.O_CREATE|os.O_APPEND, defaults.DataFilePerm)
	if err != nil {
		return fmt.Errorf("opening log file %s: %w", vmState.LogPath(), err)
	}

	logFile.Close()

	metricsFile, err := p.fs.OpenFile(vmState.MetricsPath(), os.O_WRONLY|os.O_CREATE|os.O_APPEND, defaults.DataFilePerm)
	if err != nil {
		return fmt.Errorf("opening metrics file %s: %w", vmState.MetricsPath(), err)
	}

	metricsFile.Close()

	return nil
}

// ensureSocketLink creates the socket directory of the microvm and a link to it
// in the state directory. The vsock path in the Firecracker configuration goes
// through this link. A link with another target is replaced.
func (p *fcProvider) ensureSocketLink(vmState State) error {
	socketRoot := vmState.SocketRoot()
	link := vmState.SocketLinkPath()

	if err := p.fs.MkdirAll(socketRoot, socketDirPerm); err != nil {
		return fmt.Errorf("creating socket directory %s: %w", socketRoot, err)
	}

	// The create plan may have made the directory with a wider mode.
	if err := p.fs.Chmod(socketRoot, socketDirPerm); err != nil {
		return fmt.Errorf("setting mode of socket directory %s: %w", socketRoot, err)
	}

	linker, ok := p.fs.(afero.Linker)
	if !ok {
		return fmt.Errorf("creating socket link %s: %w", link, errSymlinkNotSupported)
	}

	if reader, ok := p.fs.(afero.LinkReader); ok {
		if target, err := reader.ReadlinkIfPossible(link); err == nil && target == socketRoot {
			return nil
		}
	}

	if err := p.fs.Remove(link); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("removing stale socket link %s: %w", link, err)
	}

	if err := linker.SymlinkIfPossible(socketRoot, link); err != nil {
		return fmt.Errorf("creating socket link %s: %w", link, err)
	}

	return nil
}
