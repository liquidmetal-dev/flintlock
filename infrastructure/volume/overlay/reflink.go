package overlay

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// The FICLONE helpers below are local to this package until pkg/reflink
// lands. They stay unexported so that the move is a small change.

// cloneFile makes dst a reflink of src with FICLONE: a new inode that
// shares the data blocks of src. The mode of dst is perm. A filesystem
// that cannot clone fails with an error that wraps ErrReflinkNotSupported.
func cloneFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening %s: %w", src, err)
	}

	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_EXCL, perm)
	if err != nil {
		return fmt.Errorf("creating %s: %w", dst, err)
	}

	if err := unix.IoctlFileClone(int(out.Fd()), int(in.Fd())); err != nil {
		_ = out.Close()
		_ = os.Remove(dst)

		if errors.Is(err, unix.EOPNOTSUPP) || errors.Is(err, unix.ENOTTY) ||
			errors.Is(err, unix.EXDEV) || errors.Is(err, unix.EINVAL) {
			return fmt.Errorf("cloning %s to %s: %w: %w", src, dst, ErrReflinkNotSupported, err)
		}

		return fmt.Errorf("cloning %s to %s: %w", src, dst, err)
	}

	return out.Close()
}

// ProbeReflink checks that FICLONE works in dir. It writes a small file,
// clones it and removes both. The error wraps ErrReflinkNotSupported when
// the filesystem of dir cannot clone.
func ProbeReflink(dir string) error {
	probe, err := os.MkdirTemp(dir, ".reflink-probe-")
	if err != nil {
		return fmt.Errorf("creating probe directory in %s: %w", dir, err)
	}

	defer os.RemoveAll(probe)

	src := filepath.Join(probe, "src")
	if err := os.WriteFile(src, []byte("flintlock reflink probe\n"), lockPerm); err != nil {
		return fmt.Errorf("writing %s: %w", src, err)
	}

	return cloneFile(src, filepath.Join(probe, "dst"), lockPerm)
}
