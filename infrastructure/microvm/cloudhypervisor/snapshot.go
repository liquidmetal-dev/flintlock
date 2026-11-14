package cloudhypervisor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"

	cerrs "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/infrastructure/microvm/shared"
	"github.com/liquidmetal-dev/flintlock/pkg/cloudhypervisor"
	"github.com/liquidmetal-dev/flintlock/pkg/log"

	virtiofs "github.com/liquidmetal-dev/flintlock/infrastructure/virtiofs"
)

const (
	// snapshotFilePerm is the mode of the files of a snapshot. The memory
	// file holds all the guest memory.
	snapshotFilePerm = 0o600
	// snapshotDirPerm is the mode of the vmstate directory of a snapshot and
	// of the restore directory.
	snapshotDirPerm = 0o700

	// configFileName is the file of a cloud-hypervisor snapshot that holds the
	// config of the VM.
	configFileName = "config.json"

	// defaultAPIReadyTimeout is how long Restore waits for the API socket of a
	// new cloud-hypervisor process to answer.
	defaultAPIReadyTimeout = 10 * time.Second
	// apiPollInterval is the time between two checks of the API socket.
	apiPollInterval = 20 * time.Millisecond
)

var (
	errSnapshotPathNotAbsolute = errors.New("snapshot directory is not an absolute path")
	errSymlinkNotSupported     = errors.New("filesystem does not support symlinks")
)

// Snapshot writes the state and the memory of a paused microvm to the
// directory dest/vmstate.
func (p *provider) Snapshot(ctx context.Context, vm *models.MicroVM, dest string) error {
	log.GetLogger(ctx).WithFields(logrus.Fields{
		"service": "cloudhypervisor_microvm",
		"vmid":    vm.ID.String(),
	}).Info("taking snapshot of microvm")

	if !filepath.IsAbs(dest) {
		return fmt.Errorf("%w: %s", errSnapshotPathNotAbsolute, dest)
	}

	stateDir := filepath.Join(dest, models.SnapshotVMStateFile)

	// Cloud Hypervisor writes into an existing directory only.
	if err := p.fs.Mkdir(stateDir, snapshotDirPerm); err != nil {
		return fmt.Errorf("creating snapshot directory %s: %w", stateDir, err)
	}

	vmState := NewState(vm.ID, p.config.StateRoot, p.config.SocketDir, p.fs)
	destURL := "file://" + stateDir

	err := cloudhypervisor.New(vmState.ResolveSockPath()).Snapshot(ctx, &cloudhypervisor.VMSnapshotConfig{
		DestinationURL: &destURL,
	})
	if err != nil {
		return fmt.Errorf("creating cloud-hypervisor snapshot: %w", err)
	}

	entries, err := afero.ReadDir(p.fs, stateDir)
	if err != nil {
		return fmt.Errorf("reading snapshot directory %s: %w", stateDir, err)
	}

	for _, entry := range entries {
		path := filepath.Join(stateDir, entry.Name())

		if err := p.fs.Chmod(path, snapshotFilePerm); err != nil {
			return fmt.Errorf("setting mode of snapshot file %s: %w", path, err)
		}

		if err := p.syncPath(path); err != nil {
			return err
		}
	}

	if err := p.syncPath(stateDir); err != nil {
		return err
	}

	return p.syncPath(dest)
}

// Restore starts the cloud-hypervisor process of a microvm from the snapshot
// in src and leaves it running. The caller has created the network interfaces
// and the volumes of the microvm.
func (p *provider) Restore(ctx context.Context, vm *models.MicroVM, src string, in ports.RestoreInput) error {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"service": "cloudhypervisor_microvm",
		"vmid":    vm.ID.String(),
	})
	logger.Info("restoring microvm from snapshot")

	// Cloud Hypervisor copies the memory into the guest at restore, so the
	// restored microvm does not use the snapshot once it runs.
	if in.MemoryMode != models.MemoryModeDefault && in.MemoryMode != models.MemoryModeCopy {
		return cerrs.NewNotSupported(fmt.Sprintf("memory mode %q", in.MemoryMode))
	}

	if !filepath.IsAbs(src) {
		return fmt.Errorf("%w: %s", errSnapshotPathNotAbsolute, src)
	}

	vmState := NewState(vm.ID, p.config.StateRoot, p.config.SocketDir, p.fs)

	paths, err := p.restorePaths(vm, vmState)
	if err != nil {
		return err
	}

	if err := p.ensureState(vmState); err != nil {
		return fmt.Errorf("ensuring state dir: %w", err)
	}

	// The cloud-init image is a disk of the snapshot, so it must exist even
	// though the guest has already read it.
	if err := p.createCloudInitImage(ctx, vm, vmState); err != nil {
		return fmt.Errorf("creating metadata image: %w", err)
	}

	defer p.removeRestoreDir(logger, vmState)

	if err := p.prepareRestoreDir(src, vmState, paths); err != nil {
		return err
	}

	if vm.Spec.AllowGuestAgent {
		vm.Status.VSockPath = vmState.VSockPath()
	} else {
		vm.Status.VSockPath = ""
	}

	proc, err := p.startCloudHypervisor(restoreArgs(vmState), vmState, p.config.RunDetached)
	if err != nil {
		return fmt.Errorf("starting cloudhypervisor process: %w", err)
	}

	if err := p.resumeRestored(ctx, vmState, proc.Pid); err != nil {
		p.stopFailedRestore(logger, proc, vmState)

		return err
	}

	logger.Info("restored microvm from snapshot")

	return nil
}

// restoreArgs returns the arguments that start cloud-hypervisor from the
// snapshot in the restore directory. The snapshot holds the rest of the
// config of the microvm.
func restoreArgs(state State) []string {
	return append(apiArgs(state), "--restore", "source_url=file://"+state.RestoreDir())
}

// restorePaths returns the host paths of the devices of the restored microvm,
// in the order of the config of the snapshot.
func (p *provider) restorePaths(vm *models.MicroVM, vmState State) (restoreConfigPaths, error) {
	paths := restoreConfigPaths{
		StateRoot:  vmState.Root(),
		SocketRoot: vmState.SocketRoot(),
	}

	drives, err := shared.BuildDrives(vm, shared.DriveOptions{CloudInitImage: vmState.CloudInitImage()})
	if err != nil {
		return paths, fmt.Errorf("building drives: %w", err)
	}

	for _, drive := range drives {
		if !shared.IsDriveMounted(vm, drive) {
			return paths, cerrs.NewVolumeNotMounted(drive.VolumeID)
		}

		paths.Disks = append(paths.Disks, drive.Path)
	}

	for i := range vm.Spec.NetworkInterfaces {
		iface := &vm.Spec.NetworkInterfaces[i]

		status, ok := vm.Status.NetworkInterfaces[iface.GuestDeviceName]
		if !ok {
			return paths, cerrs.NewNetworkInterfaceStatusMissing(iface.GuestDeviceName)
		}

		// A macvtap interface is passed to cloud-hypervisor as an open file
		// descriptor, which a restore from the command line cannot pass.
		if iface.Type != models.IfaceTypeTap {
			return paths, cerrs.NewNotSupported(fmt.Sprintf("restore with a %s network interface", iface.Type))
		}

		paths.Taps = append(paths.Taps, status.HostDeviceName)
	}

	for _, vol := range vm.Spec.AdditionalVolumes {
		if vol.Source.VirtioFS == nil {
			continue
		}

		if _, ok := vm.Status.Volumes[vol.ID]; !ok {
			return paths, cerrs.NewVolumeNotMounted(vol.ID)
		}

		vfsState := virtiofs.NewState(vm.ID, p.config.StateRoot, p.config.SocketDir, p.fs)
		paths.FSSockets = append(paths.FSSockets, vfsState.ResolveVirtioFSPath())
	}

	if vm.Spec.AllowGuestAgent {
		paths.VSock = vmState.VSockPath()
	}

	return paths, nil
}

// prepareRestoreDir fills the restore directory of a microvm from the snapshot
// in src: the config with the paths of the new microvm, and links to the other
// files.
func (p *provider) prepareRestoreDir(src string, vmState State, paths restoreConfigPaths) error {
	linker, ok := p.fs.(afero.Linker)
	if !ok {
		return errSymlinkNotSupported
	}

	snapshotDir := filepath.Join(src, models.SnapshotVMStateFile)
	restoreDir := vmState.RestoreDir()

	if err := p.fs.RemoveAll(restoreDir); err != nil {
		return fmt.Errorf("removing restore directory %s: %w", restoreDir, err)
	}

	if err := p.fs.Mkdir(restoreDir, snapshotDirPerm); err != nil {
		return fmt.Errorf("creating restore directory %s: %w", restoreDir, err)
	}

	entries, err := afero.ReadDir(p.fs, snapshotDir)
	if err != nil {
		return fmt.Errorf("reading snapshot directory %s: %w", snapshotDir, err)
	}

	for _, entry := range entries {
		if entry.Name() == configFileName {
			continue
		}

		target := filepath.Join(snapshotDir, entry.Name())
		link := filepath.Join(restoreDir, entry.Name())

		if err := linker.SymlinkIfPossible(target, link); err != nil {
			return fmt.Errorf("linking snapshot file %s: %w", target, err)
		}
	}

	configPath := filepath.Join(snapshotDir, configFileName)

	config, err := afero.ReadFile(p.fs, configPath)
	if err != nil {
		return fmt.Errorf("reading snapshot config %s: %w", configPath, err)
	}

	config, err = rewriteRestoreConfig(config, paths)
	if err != nil {
		return fmt.Errorf("rewriting snapshot config %s: %w", configPath, err)
	}

	restoreConfig := filepath.Join(restoreDir, configFileName)
	if err := afero.WriteFile(p.fs, restoreConfig, config, snapshotFilePerm); err != nil {
		return fmt.Errorf("writing restore config %s: %w", restoreConfig, err)
	}

	return nil
}

func (p *provider) removeRestoreDir(logger *logrus.Entry, vmState State) {
	if err := p.fs.RemoveAll(vmState.RestoreDir()); err != nil {
		logger.WithError(err).Warnf("removing restore directory %s", vmState.RestoreDir())
	}
}

// resumeRestored saves the pid of the restored cloud-hypervisor process and
// resumes the microvm once the API socket answers. A restored microvm starts
// paused.
func (p *provider) resumeRestored(ctx context.Context, vmState State, pid int) error {
	if err := vmState.SetPid(pid); err != nil {
		return fmt.Errorf("saving pid %d to file: %w", pid, err)
	}

	client := cloudhypervisor.New(vmState.SockPath())

	if err := p.waitForAPI(ctx, client); err != nil {
		return err
	}

	if err := client.Resume(ctx); err != nil {
		return fmt.Errorf("resuming cloud-hypervisor microvm: %w", err)
	}

	return nil
}

// waitForAPI waits until the API socket of a new cloud-hypervisor process
// answers.
func (p *provider) waitForAPI(ctx context.Context, client cloudhypervisor.Client) error {
	timeout := p.apiReadyTimeout
	if timeout == 0 {
		timeout = defaultAPIReadyTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(apiPollInterval)
	defer ticker.Stop()

	for {
		_, err := client.VmmPing(ctx)
		if err == nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("waiting for cloud-hypervisor api socket: %w", err)
		case <-ticker.C:
		}
	}
}

// stopFailedRestore kills the cloud-hypervisor process of a failed restore and
// removes its pid file and API socket. The restore plan removes the rest.
func (p *provider) stopFailedRestore(logger *logrus.Entry, proc *os.Process, vmState State) {
	if err := proc.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		logger.WithError(err).Warnf("killing cloud-hypervisor process %d", proc.Pid)
	}

	// A detached process is reaped by the goroutine of process.DetachedStart.
	if !p.config.RunDetached {
		_, _ = proc.Wait()
	}

	for _, path := range []string{vmState.PIDPath(), vmState.SockPath()} {
		if err := p.fs.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			logger.WithError(err).Warnf("removing %s", path)
		}
	}
}

// syncPath flushes a file or a directory to disk.
func (p *provider) syncPath(path string) error {
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
