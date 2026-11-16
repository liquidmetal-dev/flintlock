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
	digest, err := c.in.Images.Digest(ctx, &ports.ImageSpec{ImageName: ref, Owner: c.in.VM.ID.String()})
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
