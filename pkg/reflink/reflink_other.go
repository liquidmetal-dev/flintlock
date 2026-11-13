//go:build !linux

package reflink

import (
	"fmt"
	"os"
)

// Probe always fails: FICLONE is a Linux ioctl.
func Probe(dir string) error {
	return fmt.Errorf("%w: %s: not on linux", ErrNotSupported, dir)
}

// Clone always fails: FICLONE is a Linux ioctl.
func Clone(src, dst string, _ os.FileMode) error {
	return fmt.Errorf("cloning %s to %s: %w: not on linux", src, dst, ErrNotSupported)
}

// FilesystemType always fails: statfs magic numbers are Linux specific.
func FilesystemType(path string) (string, error) {
	return "", fmt.Errorf("filesystem type of %s: not supported on this platform", path)
}
