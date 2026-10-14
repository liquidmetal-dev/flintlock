//go:build !linux

package reflink

import "fmt"

// Probe always fails: FICLONE is a Linux ioctl.
func Probe(dir string) error {
	return fmt.Errorf("%w: %s: not on linux", ErrNotSupported, dir)
}

// FilesystemType always fails: statfs magic numbers are Linux specific.
func FilesystemType(path string) (string, error) {
	return "", fmt.Errorf("filesystem type of %s: not supported on this platform", path)
}
