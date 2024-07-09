package godisk

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"

	"github.com/diskfs/go-diskfs"
	"github.com/diskfs/go-diskfs/disk"
	"github.com/diskfs/go-diskfs/filesystem"
	units "github.com/docker/go-units"
	"github.com/spf13/afero"

	"github.com/liquidmetal-dev/flintlock/core/ports"
)

const (
	defaultBlockSizeInBytes = 512
	fillVolume              = 0
)

func New(fs afero.Fs) ports.DiskService {
	return &diskService{
		fs: fs,
	}
}

type diskService struct {
	fs afero.Fs
}

// Create will create a new disk.
func (s *diskService) Create(ctx context.Context, input ports.DiskCreateInput) error {
	if input.Path == "" {
		return errPathRequired
	}
	if input.Size == "" {
		return errSizeRequired
	}

	imageExists, err := s.imageExists(input.Path)
	if err != nil {
		return fmt.Errorf("checking if disk exists: %w", err)
	}

	if imageExists {
		if !input.Overwrite {
			return os.ErrExist
		}
		if removeErr := s.fs.Remove(input.Path); removeErr != nil {
			return fmt.Errorf("removing disk %s: %w", input.Path, removeErr)
		}
	}

	diskSize, err := units.FromHumanSize(input.Size)
	if err != nil {
		return fmt.Errorf("converting disk size %s: %w", input.Size, err)
	}

	createdDisk, err := diskfs.Create(input.Path, int64(diskSize), diskfs.Raw, diskfs.SectorSizeDefault)
	if err != nil {
		return fmt.Errorf("creating disk %s: %w", input.Path, err)
	}

	createdDisk.LogicalBlocksize = defaultBlockSizeInBytes
	fspec := disk.FilesystemSpec{
		Partition:   fillVolume,
		FSType:      filesystem.Type(input.Type),
		VolumeLabel: input.VolumeName,
	}
	fs, err := createdDisk.CreateFilesystem(fspec)
	if err != nil {
		return fmt.Errorf("creating filesystem on %s: %w", input.Path, err)
	}

	for _, file := range input.Files {
		if writeErr := createFileInImage(file.Path, file.ContentBase64, fs); writeErr != nil {
			return fmt.Errorf("creating file %s in image: %w", input.Path, writeErr)
		}
	}

	return nil
}

func (s *diskService) imageExists(path string) (bool, error) {
	if _, err := s.fs.Stat(path); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return false, err
		}

		return false, nil
	}

	return true, nil
}

func createFileInImage(dest string, content string, fs filesystem.FileSystem) error {
	decoded, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return fmt.Errorf("base64 decoding content %s: %w", content, err)
	}

	rw, err := fs.OpenFile(dest, os.O_CREATE|os.O_RDWR)
	if err != nil {
		return err
	}

	_, err = rw.Write(decoded)

	return err
}
