package overlay

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
)

// Capture reflinks the writable disk of the microvm to root.rw in destDir
// and links root.base in destDir to the base of the microvm, relative to
// destDir. The link is the snapshot's reference to the base: the sweep keeps
// a base that a snapshots/*/root.base link names. A root.rw that exists in
// destDir is kept, and so is the base its root.base names.
func (s *Service) Capture(ctx context.Context, ref ports.VolumeRef, destDir string) (*models.CapturedVolume, error) {
	logger := s.logger(ctx, ref)

	if err := os.MkdirAll(destDir, dirPerm); err != nil {
		return nil, fmt.Errorf("creating %s: %w", destDir, err)
	}

	srcDir := s.volumesDir(ref.VMID)
	linkPath := filepath.Join(destDir, baseLinkName)

	// The link is made before the disk, so a root.rw in destDir always has
	// a root.base next to it that names the base it was written over.
	sum, err := s.linkedBase(linkPath)
	if errors.Is(err, os.ErrNotExist) {
		sum, err = s.linkedBase(filepath.Join(srcDir, baseLinkName))
	}

	if err != nil {
		return nil, err
	}

	basePath := s.basePath(sum)

	if _, err := os.Stat(basePath); err != nil {
		return nil, fmt.Errorf("base %s of %s: %w", sum, ref.VMID, err)
	}

	if err := linkBase(linkPath, basePath); err != nil {
		return nil, err
	}

	rwPath := filepath.Join(destDir, rwDiskName)

	made, err := s.cloneDisk(filepath.Join(srcDir, rwDiskName), rwPath)
	if err != nil {
		return nil, err
	}

	if made {
		logger.Infof("captured writable disk to %s on base %s", rwPath, sum)
	} else {
		logger.Debugf("captured disk %s exists, kept", rwPath)
	}

	info, err := os.Stat(rwPath)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", rwPath, err)
	}

	var fs unix.Statfs_t
	if err := unix.Statfs(destDir, &fs); err != nil {
		return nil, fmt.Errorf("statfs %s: %w", destDir, err)
	}

	key, err := s.baseKeyOf(sum)
	if err != nil {
		return nil, err
	}

	return &models.CapturedVolume{
		BaseKey:    key,
		BaseDigest: sum,
		SizeBytes:  info.Size(),
		BlockSize:  int64(fs.Bsize), //nolint:unconvert // Bsize is not int64 on every platform
	}, nil
}

// Restore reflinks root.rw of the snapshot directory to the writable disk
// of the microvm and links the microvm's root.base to the base that the
// snapshot's root.base names, as Create does. A root.rw that exists is
// kept. The last use of the base is refreshed. The snapshot directory is
// not changed.
func (s *Service) Restore(ctx context.Context, in ports.VolumeRestoreInput) (*ports.VolumeMounts, error) {
	logger := s.logger(ctx, in.VolumeRef)

	sum, err := s.linkedBase(filepath.Join(in.SnapshotDir, baseLinkName))
	if err != nil {
		return nil, err
	}

	basePath := s.basePath(sum)

	if _, err := os.Stat(basePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, &BaseMissingError{Digest: sum, Snapshot: filepath.Base(filepath.Clean(in.SnapshotDir))}
		}

		return nil, fmt.Errorf("stat %s: %w", basePath, err)
	}

	if err := s.touchBase(ctx, sum); err != nil {
		return nil, err
	}

	dir := s.volumesDir(in.VMID)

	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return nil, fmt.Errorf("creating %s: %w", dir, err)
	}

	linkPath := filepath.Join(dir, baseLinkName)

	if err := linkBase(linkPath, basePath); err != nil {
		return nil, err
	}

	rwPath := filepath.Join(dir, rwDiskName)

	made, err := s.cloneDisk(filepath.Join(in.SnapshotDir, rwDiskName), rwPath)
	if err != nil {
		return nil, err
	}

	if made {
		logger.Infof("restored writable disk %s from %s on base %s", rwPath, in.SnapshotDir, sum)
	} else {
		logger.Debugf("writable disk %s exists, kept", rwPath)
	}

	return &ports.VolumeMounts{
		BaseMount:  models.Mount{Type: models.MountTypeFile, Source: linkPath},
		RWMount:    models.Mount{Type: models.MountTypeFile, Source: rwPath},
		BaseDigest: sum,
	}, nil
}

// linkedBase returns the digest of the base that the root.base link names.
// The error wraps os.ErrNotExist when there is no link.
func (s *Service) linkedBase(link string) (string, error) {
	target, err := os.Readlink(link)
	if err != nil {
		return "", fmt.Errorf("reading link %s: %w", link, err)
	}

	sum, err := digestFromPath(target)
	if err != nil {
		return "", fmt.Errorf("link %s: %w", link, err)
	}

	return sum, nil
}

// basePath is the path of the base file with the digest on this host.
func (s *Service) basePath(sum string) string {
	return filepath.Join(s.root, basesDir, sum+baseExt)
}

// baseKeyOf returns the key of the first by-source link to the base with
// the digest, or "" when no link names it.
func (s *Service) baseKeyOf(sum string) (string, error) {
	entries, err := s.index.Entries()
	if err != nil {
		return "", err
	}

	for _, entry := range entries {
		if entry.Digest == sum {
			return entry.Key.String(), nil
		}
	}

	return "", nil
}

// touchBase refreshes the last use of every by-source link to the base with
// the digest, under the lock of its key, as a Create of the image would.
func (s *Service) touchBase(ctx context.Context, sum string) error {
	entries, err := s.index.Entries()
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.Digest != sum {
			continue
		}

		if err := s.touchEntry(ctx, entry.Key); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) touchEntry(ctx context.Context, key baseKey) error {
	release, err := s.index.Lock(ctx, key)
	if err != nil {
		return err
	}

	defer func() { _ = release() }()

	if err := s.index.Touch(key); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return nil
}

// exists reports whether path exists, without following a link.
func exists(path string) (bool, error) {
	_, err := os.Lstat(path)
	if err == nil {
		return true, nil
	}

	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}

	return false, fmt.Errorf("stat %s: %w", path, err)
}
