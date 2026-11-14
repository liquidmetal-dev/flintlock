//go:build linux

package reflink

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// Probe creates a small file in dir, clones it to a second file with
// FICLONE and removes both. It returns nil when the clone succeeds and an
// error wrapping ErrNotSupported when the filesystem refuses the ioctl.
func Probe(dir string) error {
	src, err := os.CreateTemp(dir, ".reflink-probe-src-*")
	if err != nil {
		return fmt.Errorf("creating reflink probe file in %s: %w", dir, err)
	}
	defer os.Remove(src.Name())
	defer src.Close()

	if _, err := src.WriteString("reflink probe\n"); err != nil {
		return fmt.Errorf("writing reflink probe file %s: %w", src.Name(), err)
	}

	dst, err := os.CreateTemp(dir, ".reflink-probe-dst-*")
	if err != nil {
		return fmt.Errorf("creating reflink probe file in %s: %w", dir, err)
	}
	defer os.Remove(dst.Name())
	defer dst.Close()

	if err := unix.IoctlFileClone(int(dst.Fd()), int(src.Fd())); err != nil {
		if isNotSupported(err) {
			return fmt.Errorf("%w: %s: %w", ErrNotSupported, dir, err)
		}

		return fmt.Errorf("cloning reflink probe file in %s: %w", dir, err)
	}

	return nil
}

// isNotSupported reports whether err is one of the errno values the kernel
// returns when FICLONE is not available on the filesystem.
func isNotSupported(err error) bool {
	return errors.Is(err, unix.EOPNOTSUPP) ||
		errors.Is(err, unix.ENOTTY) ||
		errors.Is(err, unix.EINVAL) ||
		errors.Is(err, unix.EXDEV)
}

// FilesystemType returns the name of the filesystem that holds path, such
// as "xfs" or "btrfs". Types the package does not know are returned as
// their statfs magic number in hex.
func FilesystemType(path string) (string, error) {
	var stat unix.Statfs_t

	if err := unix.Statfs(path, &stat); err != nil {
		return "", fmt.Errorf("statfs %s: %w", path, err)
	}

	switch uint32(stat.Type) {
	case unix.XFS_SUPER_MAGIC:
		return FilesystemXFS, nil
	case unix.BTRFS_SUPER_MAGIC:
		return FilesystemBtrfs, nil
	case unix.EXT4_SUPER_MAGIC:
		return FilesystemExt4, nil
	case unix.TMPFS_MAGIC:
		return FilesystemTmpfs, nil
	case unix.OVERLAYFS_SUPER_MAGIC:
		return FilesystemOverlay, nil
	default:
		return fmt.Sprintf("0x%x", stat.Type), nil
	}
}
