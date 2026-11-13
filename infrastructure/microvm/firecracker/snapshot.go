package firecracker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"

	cerrs "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/pkg/firecracker"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
)

const (
	// snapshotFilePerm is the mode of the files of a snapshot. The memory
	// file holds all the guest memory.
	snapshotFilePerm = 0o600

	// defaultAPIReadyTimeout is how long Restore waits for the API socket of a
	// new firecracker process to answer.
	defaultAPIReadyTimeout = 10 * time.Second
	// apiPollInterval is the time between two checks of the API socket.
	apiPollInterval = 20 * time.Millisecond
)

var errSnapshotPathNotAbsolute = errors.New("snapshot directory is not an absolute path")

// Snapshot writes the state and the memory of a paused microvm to
// dest/vmstate and dest/memory.
func (p *fcProvider) Snapshot(ctx context.Context, vm *models.MicroVM, dest string) error {
	log.GetLogger(ctx).WithFields(logrus.Fields{
		"service": "firecracker_microvm",
		"vmid":    vm.ID.String(),
	}).Info("taking snapshot of microvm")

	// The working directory of firecracker is the state directory of the
	// microvm, so a relative path would resolve there.
	if !filepath.IsAbs(dest) {
		return fmt.Errorf("%w: %s", errSnapshotPathNotAbsolute, dest)
	}

	vmState := NewState(vm.ID, p.config.StateRoot, p.config.SocketDir, p.fs)
	statePath := filepath.Join(dest, models.SnapshotVMStateFile)
	memoryPath := filepath.Join(dest, models.SnapshotMemoryFile)

	// Firecracker v1.16 refuses sync_snapshot_files, so the files are synced
	// below.
	err := firecracker.New(vmState.APISockPath()).CreateSnapshot(ctx, firecracker.SnapshotCreateParams{
		SnapshotType: firecracker.SnapshotTypeFull,
		SnapshotPath: statePath,
		MemFilePath:  memoryPath,
	})
	if err != nil {
		return fmt.Errorf("creating firecracker snapshot: %w", err)
	}

	for _, path := range []string{statePath, memoryPath} {
		if err := p.fs.Chmod(path, snapshotFilePerm); err != nil {
			return fmt.Errorf("setting mode of snapshot file %s: %w", path, err)
		}

		if err := p.syncPath(path); err != nil {
			return err
		}
	}

	return p.syncPath(dest)
}

// Restore starts the firecracker process of a microvm from the snapshot in
// src and leaves it running. The caller has created the network interfaces
// and the volumes of the microvm.
func (p *fcProvider) Restore(ctx context.Context, vm *models.MicroVM, src string, in ports.RestoreInput) error {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"service": "firecracker_microvm",
		"vmid":    vm.ID.String(),
	})
	logger.Info("restoring microvm from snapshot")

	// Firecracker maps the memory file, so it stays in the snapshot directory.
	if in.MemoryMode != models.MemoryModeDefault && in.MemoryMode != models.MemoryModeFile {
		return cerrs.NewNotSupported(fmt.Sprintf("memory mode %q", in.MemoryMode))
	}

	if !filepath.IsAbs(src) {
		return fmt.Errorf("%w: %s", errSnapshotPathNotAbsolute, src)
	}

	overrides, err := networkOverrides(vm)
	if err != nil {
		return err
	}

	vmState := NewState(vm.ID, p.config.StateRoot, p.config.SocketDir, p.fs)

	if err := p.ensureState(vmState); err != nil {
		return fmt.Errorf("ensuring state dir: %w", err)
	}

	if vm.Spec.AllowGuestAgent {
		vm.Status.VSockPath = vmState.VSockPath()
	} else {
		vm.Status.VSockPath = ""
	}

	proc, err := p.startFirecracker(p.restoreCommand(vm, vmState), vmState, p.config.RunDetached)
	if err != nil {
		return fmt.Errorf("starting firecracker process: %w", err)
	}

	if err := p.loadSnapshot(ctx, vm, vmState, proc.Pid, src, overrides); err != nil {
		p.stopFailedRestore(logger, proc, vmState)

		return err
	}

	logger.Info("restored microvm from snapshot")

	return nil
}

// restoreCommand returns the command that starts firecracker for a restore.
// It has no config file and no metadata: the snapshot holds the microvm. The
// logger and metrics settings of the config file go on the command line.
func (p *fcProvider) restoreCommand(vm *models.MicroVM, vmState State) *exec.Cmd {
	logger := loggerConfig(vmState)

	args := []string{"--log-path", logger.LogPath, "--level", string(logger.Level)}

	if logger.ShowLevel {
		args = append(args, "--show-level")
	}

	if logger.ShowLogOrigin {
		args = append(args, "--show-log-origin")
	}

	args = append(args, "--metrics-path", metricsConfig(vmState).Path)

	return p.vmmCommand(vm, vmState, args...)
}

// loadSnapshot loads the snapshot in src into the firecracker process with pid,
// puts the metadata of the microvm and resumes it.
func (p *fcProvider) loadSnapshot(
	ctx context.Context,
	vm *models.MicroVM,
	vmState State,
	pid int,
	src string,
	overrides []firecracker.NetworkOverride,
) error {
	if err := vmState.SetPid(pid); err != nil {
		return fmt.Errorf("saving pid %d to file: %w", pid, err)
	}

	client := firecracker.New(vmState.APISockPath())

	if err := p.waitForAPI(ctx, client); err != nil {
		return err
	}

	err := client.LoadSnapshot(ctx, firecracker.SnapshotLoadParams{
		SnapshotPath: filepath.Join(src, models.SnapshotVMStateFile),
		MemBackend: firecracker.MemoryBackend{
			BackendType: firecracker.MemoryBackendTypeFile,
			BackendPath: filepath.Join(src, models.SnapshotMemoryFile),
		},
		NetworkOverrides: overrides,
		ResumeVM:         false,
	})
	if err != nil {
		return fmt.Errorf("loading firecracker snapshot: %w", err)
	}

	// Firecracker keeps the MMDS config in a snapshot but not the data. The
	// put comes before the resume, so the guest never sees the data of the
	// source.
	if err := client.PutMMDS(ctx, decodeMetadata(vm.Spec.Metadata)); err != nil {
		return fmt.Errorf("putting firecracker metadata: %w", err)
	}

	if err := client.Resume(ctx); err != nil {
		return fmt.Errorf("resuming firecracker microvm: %w", err)
	}

	return nil
}

// waitForAPI waits until the API socket of a new firecracker process answers.
func (p *fcProvider) waitForAPI(ctx context.Context, client firecracker.Client) error {
	timeout := p.apiReadyTimeout
	if timeout == 0 {
		timeout = defaultAPIReadyTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(apiPollInterval)
	defer ticker.Stop()

	for {
		_, err := client.Describe(ctx)
		if err == nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("waiting for firecracker api socket: %w", err)
		case <-ticker.C:
		}
	}
}

// stopFailedRestore kills the firecracker process of a failed restore and
// removes its pid file and API socket. The restore plan removes the rest.
func (p *fcProvider) stopFailedRestore(logger *logrus.Entry, proc *os.Process, vmState State) {
	if err := proc.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		logger.WithError(err).Warnf("killing firecracker process %d", proc.Pid)
	}

	// A detached process is reaped by the goroutine of process.DetachedStart.
	if !p.config.RunDetached {
		_, _ = proc.Wait()
	}

	for _, path := range []string{vmState.PIDPath(), vmState.APISockPath()} {
		if err := p.fs.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			logger.WithError(err).Warnf("removing %s", path)
		}
	}
}

// networkOverrides maps the identifier of each network interface in the
// snapshot to the host device of the restored microvm. The identifiers and
// device names are the ones of the Firecracker config.
func networkOverrides(vm *models.MicroVM) ([]firecracker.NetworkOverride, error) {
	overrides := make([]firecracker.NetworkOverride, 0, len(vm.Spec.NetworkInterfaces))

	for i := range vm.Spec.NetworkInterfaces {
		iface := &vm.Spec.NetworkInterfaces[i]

		status, ok := vm.Status.NetworkInterfaces[iface.GuestDeviceName]
		if !ok {
			return nil, cerrs.NewNetworkInterfaceStatusMissing(iface.GuestDeviceName)
		}

		fcIface := createNetworkIface(iface, status)
		overrides = append(overrides, firecracker.NetworkOverride{
			IfaceID:     fcIface.IfaceID,
			HostDevName: fcIface.HostDevName,
		})
	}

	return overrides, nil
}

// syncPath flushes a file or a directory to disk.
func (p *fcProvider) syncPath(path string) error {
	file, err := p.fs.Open(path)
	if err != nil {
		return fmt.Errorf("opening %s: %w", path, err)
	}

	defer file.Close()

	if err := file.Sync(); err != nil {
		return fmt.Errorf("syncing %s: %w", path, err)
	}

	return nil
}
