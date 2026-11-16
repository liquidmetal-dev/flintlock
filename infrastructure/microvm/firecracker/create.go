package firecracker

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"

	"github.com/firecracker-microvm/firecracker-go-sdk"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"

	"github.com/liquidmetal-dev/flintlock/client/cloudinit"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/infrastructure/microvm/shared"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
	"github.com/liquidmetal-dev/flintlock/pkg/process"
)

// defaultVendorData is served when the spec has no vendor-data. Without it
// MMDS returns 404 and cloud-init retries the URL for about 10 seconds.
const defaultVendorData = "#cloud-config\n{}\n"

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

	config, err := p.vmmConfig(vm, vmState)
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

	if err = writeMetadata(vm, vmState); err != nil {
		return err
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

func (p *fcProvider) vmmConfig(vm *models.MicroVM, vmState State) (*VmmConfig, error) {
	opts := []MicroVMOption{}
	if p.config.MetadataKernelIP {
		opts = append(opts, WithMetadataKernelIP())
	}

	return CreateConfig(WithMicroVM(vm, opts...), WithState(vmState), WithVsock(vm, vmState))
}

// command returns the command that starts firecracker for a microvm. The
// working directory of the process is the state directory of the microvm, so
// that the relative drive paths of an overlay root resolve to its own disks.
func (p *fcProvider) command(vm *models.MicroVM, vmState State) *exec.Cmd {
	args := []string{"--id", vm.ID.UID(), "--boot-timer", "--no-api"}
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

	// Remove any stale guest-agent vsock socket so the VMM can bind on (re)create. The VMM isn't
	// running, so a socket left in the state dir by an older flintlock can go too.
	if err := shared.RemoveStaleSockets(p.fs, vmState.VSockPath(), vmState.legacyVSockPath()); err != nil {
		return fmt.Errorf("removing stale vsock socket: %w", err)
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

func writeMetadata(vm *models.MicroVM, vmState State) error {
	if err := vmState.SetMetadata(&Metadata{Latest: mmdsMetadata(vm.Spec.Metadata)}); err != nil {
		return fmt.Errorf("saving firecracker metadata: %w", err)
	}

	return nil
}

func mmdsMetadata(specMetadata map[string]string) map[string]string {
	latest := make(map[string]string, len(specMetadata)+1)
	for key, value := range specMetadata {
		latest[key] = value
	}

	if _, ok := latest[cloudinit.VendorDataKey]; !ok {
		latest[cloudinit.VendorDataKey] = base64.StdEncoding.EncodeToString([]byte(defaultVendorData))
	}

	return latest
}
