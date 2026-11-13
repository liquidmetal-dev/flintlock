//go:build linux

package overlayinit

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
)

// NewSystem returns the System that calls the real Linux syscalls.
func NewSystem() System {
	return linuxSystem{}
}

type linuxSystem struct{}

func (linuxSystem) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (linuxSystem) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func (linuxSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (linuxSystem) Mount(source, target, fstype string, flags uintptr, data string) error {
	return unix.Mount(source, target, fstype, flags, data)
}

func (linuxSystem) Chdir(path string) error {
	return unix.Chdir(path)
}

func (linuxSystem) Chroot(path string) error {
	return unix.Chroot(path)
}

func (linuxSystem) RemoveContents(dir string) error {
	var stat unix.Stat_t
	if err := unix.Lstat(dir, &stat); err != nil {
		return fmt.Errorf("stat %s: %w", dir, err)
	}

	return removeContents(dir, stat.Dev)
}

// removeContents deletes the entries of dir that live on the device dev.
// An entry on another device is a mount point and is left alone, together
// with everything under it.
func removeContents(dir string, dev uint64) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read %s: %w", dir, err)
	}

	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())

		var stat unix.Stat_t
		if err := unix.Lstat(path, &stat); err != nil {
			return fmt.Errorf("stat %s: %w", path, err)
		}

		if stat.Dev != dev {
			continue
		}

		if entry.IsDir() {
			if err := removeContents(path, dev); err != nil {
				return err
			}

			if err := unix.Rmdir(path); err != nil {
				return fmt.Errorf("rmdir %s: %w", path, err)
			}

			continue
		}

		if err := unix.Unlink(path); err != nil {
			return fmt.Errorf("unlink %s: %w", path, err)
		}
	}

	return nil
}

func (linuxSystem) Environ() []string {
	return os.Environ()
}

func (linuxSystem) Exec(path string, argv, env []string) error {
	return unix.Exec(path, argv, env)
}

func (linuxSystem) Now() time.Time {
	return time.Now()
}

func (linuxSystem) Sleep(d time.Duration) {
	time.Sleep(d)
}
