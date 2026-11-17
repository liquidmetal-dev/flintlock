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
	"github.com/liquidmetal-dev/flintlock/core/ports"
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
	errNoImageDigest    = errors.New("the image service returned no digest")
)

func (c *Capture) writeFiles(ctx context.Context) error {
	desc, err := c.descriptor(ctx)
	if err != nil {
		return err
	}

	if err := c.writeJSON(models.SnapshotDescriptorFilename, desc); err != nil {
		return err
	}

	return c.writeJSON(models.SnapshotSpecFilename, c.vm.Spec)
}

// descriptor returns the compatibility descriptor of the snapshot. It has no
// field for the metadata of the source, which often holds secrets.
func (c *Capture) descriptor(ctx context.Context) (*models.SnapshotDescriptor, error) {
	if c.volume == nil {
		return nil, errNoCapturedVolume
	}

	vm := c.vm

	facts, err := c.in.HostInfo.Facts(ctx, vm.Spec.Provider)
	if err != nil {
		return nil, fmt.Errorf("getting the host facts: %w", err)
	}

	root, err := c.rootVolume()
	if err != nil {
		return nil, err
	}

	kernel, err := c.image(ctx, string(vm.Spec.Kernel.Image))
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
		KernelImage:       *kernel,
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
		desc.InitrdImage, err = c.image(ctx, string(vm.Spec.Initrd.Image))
		if err != nil {
			return nil, err
		}
	}

	return desc, nil
}

// image returns the reference and the digest of an image the source booted
// from. A restore needs the exact image, as a tag can move to other content.
func (c *Capture) image(ctx context.Context, ref string) (*models.SnapshotImage, error) {
	digest, err := c.in.Images.Digest(ctx, &ports.ImageSpec{ImageName: ref, Owner: c.vm.ID.String()})
	if err != nil {
		return nil, fmt.Errorf("getting the digest of image %s: %w", ref, err)
	}

	if digest == "" {
		return nil, fmt.Errorf("image %s: %w", ref, errNoImageDigest)
	}

	return &models.SnapshotImage{Reference: ref, Digest: digest}, nil
}

// rootVolume describes the captured writable disk of the root, at the
// position the guest sees it.
func (c *Capture) rootVolume() (models.SnapshotVolume, error) {
	drives, err := c.in.Provider.Drives(c.vm)
	if err != nil {
		return models.SnapshotVolume{}, fmt.Errorf("getting the drives of microvm %s: %w", c.vm.ID, err)
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

	return models.SnapshotVolume{}, fmt.Errorf("microvm %s: %w", c.vm.ID, errNoRootRWDrive)
}

// writeJSON writes value as JSON to the file with the name in the snapshot
// directory, readable by the owner only. The bytes go to a new 0600 file that
// is renamed over the destination, so they are never in a file with a wider
// mode, such as one an earlier run left behind.
func (c *Capture) writeJSON(name string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encoding %s: %w", name, err)
	}

	path := filepath.Join(c.in.Dir, name)

	tmp, err := afero.TempFile(c.in.FS, c.in.Dir, "."+name+".tmp-*")
	if err != nil {
		return fmt.Errorf("creating a temporary file for %s: %w", path, err)
	}

	tmpPath := tmp.Name()

	if err := writeSynced(c.in.FS, tmp, data); err != nil {
		_ = c.in.FS.Remove(tmpPath)

		return fmt.Errorf("writing %s: %w", tmpPath, err)
	}

	if err := c.in.FS.Rename(tmpPath, path); err != nil {
		_ = c.in.FS.Remove(tmpPath)

		return fmt.Errorf("renaming %s to %s: %w", tmpPath, path, err)
	}

	return syncDir(c.in.FS, c.in.Dir)
}

// writeSynced sets the mode of the file to 0600 before it writes data, and
// syncs and closes it.
func writeSynced(fs afero.Fs, f afero.File, data []byte) error {
	defer func() { _ = f.Close() }()

	if err := fs.Chmod(f.Name(), filePerm); err != nil {
		return fmt.Errorf("setting the mode: %w", err)
	}

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("writing: %w", err)
	}

	if err := f.Sync(); err != nil {
		return fmt.Errorf("syncing: %w", err)
	}

	return f.Close() //nolint:wrapcheck // the caller adds the path
}

func syncDir(fs afero.Fs, dir string) error {
	d, err := fs.Open(dir)
	if err != nil {
		return fmt.Errorf("opening %s: %w", dir, err)
	}

	defer func() { _ = d.Close() }()

	if err := d.Sync(); err != nil {
		return fmt.Errorf("syncing %s: %w", dir, err)
	}

	return nil
}
