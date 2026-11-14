package snapshot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/afero"

	"github.com/liquidmetal-dev/flintlock/core/models"
)

const (
	filePerm os.FileMode = 0o600

	// writableDiskFilesystem is the filesystem of the writable disk of an
	// overlay root, which is the only volume a snapshot captures.
	writableDiskFilesystem = "ext4"
)

var (
	errNoCapturedVolume = errors.New("the root volume has not been captured")
	errNoRootRWDrive    = errors.New("no drive holds the writable disk of the root volume")
)

func (c *Capture) writeFiles(ctx context.Context) error {
	desc, err := c.descriptor(ctx)
	if err != nil {
		return err
	}

	if err := c.writeJSON(models.SnapshotDescriptorFilename, desc); err != nil {
		return err
	}

	return c.writeJSON(models.SnapshotSpecFilename, c.in.VM.Spec)
}

// descriptor returns the compatibility descriptor of the snapshot. It has no
// field for the metadata of the source, which often holds secrets.
func (c *Capture) descriptor(ctx context.Context) (*models.SnapshotDescriptor, error) {
	if c.volume == nil {
		return nil, errNoCapturedVolume
	}

	vm := c.in.VM

	facts, err := c.in.HostInfo.Facts(ctx, vm.Spec.Provider)
	if err != nil {
		return nil, fmt.Errorf("getting the host facts: %w", err)
	}

	root, err := c.rootVolume()
	if err != nil {
		return nil, err
	}

	desc := &models.SnapshotDescriptor{
		FormatVersion:     models.SnapshotDescriptorFormatVersion,
		VMMType:           facts.VMMType,
		VMMVersion:        facts.VMMVersion,
		FCSnapshotVersion: facts.FCSnapshotVersion,
		FlintlockVersion:  c.in.FlintlockVersion,
		Arch:              facts.Arch,
		CPUVendor:         facts.CPUVendor,
		CPUModel:          facts.CPUModel,
		CPUMicrocode:      facts.CPUMicrocode,
		HostKernel:        facts.HostKernel,
		VCPUs:             vm.Spec.VCPU,
		MemoryMB:          vm.Spec.MemoryInMb,
		KernelImage:       models.SnapshotImage{Reference: string(vm.Spec.Kernel.Image)},
		Volumes:           []models.SnapshotVolume{root},
		CreatedAt:         c.pausedAt,
		Source: models.SnapshotOrigin{
			UID:       vm.ID.UID(),
			Name:      vm.ID.Name(),
			Namespace: vm.ID.Namespace(),
		},
		Labels: c.in.Snapshot.Labels,
	}

	if vm.Spec.Initrd != nil {
		desc.InitrdImage = &models.SnapshotImage{Reference: string(vm.Spec.Initrd.Image)}
	}

	return desc, nil
}

// rootVolume describes the captured writable disk of the root, at the
// position the guest sees it.
func (c *Capture) rootVolume() (models.SnapshotVolume, error) {
	drives, err := c.in.Provider.Drives(c.in.VM)
	if err != nil {
		return models.SnapshotVolume{}, fmt.Errorf("getting the drives of microvm %s: %w", c.in.VM.ID, err)
	}

	for order, drive := range drives {
		if drive.Role != models.DriveRoleRootRW {
			continue
		}

		return models.SnapshotVolume{
			DriveID:    drive.ID,
			Order:      order,
			SizeBytes:  c.volume.SizeBytes,
			BlockSize:  c.volume.BlockSize,
			Filesystem: writableDiskFilesystem,
			BaseDigest: "sha256:" + c.volume.BaseDigest,
		}, nil
	}

	return models.SnapshotVolume{}, fmt.Errorf("microvm %s: %w", c.in.VM.ID, errNoRootRWDrive)
}

// writeJSON writes value as JSON to the file with the name in the snapshot
// directory, readable by the owner only.
func (c *Capture) writeJSON(name string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encoding %s: %w", name, err)
	}

	path := filepath.Join(c.in.Dir, name)

	if err := afero.WriteFile(c.in.FS, path, data, filePerm); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	// WriteFile does not change the mode of a file that exists, and the umask
	// applies to one it creates.
	if err := c.in.FS.Chmod(path, filePerm); err != nil {
		return fmt.Errorf("setting the mode of %s: %w", path, err)
	}

	return nil
}
