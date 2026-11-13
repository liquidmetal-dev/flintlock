// Package reflink clones files with the FICLONE ioctl and checks whether a
// directory is on a filesystem that can. The provisioner uses it to verify
// a state directory, the daemon uses it to probe the host at start, and
// the overlay volume service clones writable disks with it.
package reflink

import "errors"

// ErrNotSupported is returned by Probe and Clone when the filesystem does
// not support FICLONE.
var ErrNotSupported = errors.New("filesystem does not support reflink (FICLONE)")

// Filesystem names returned by FilesystemType for the types the
// provisioner cares about.
const (
	FilesystemXFS   = "xfs"
	FilesystemBtrfs = "btrfs"
	FilesystemExt4  = "ext4"
	FilesystemTmpfs = "tmpfs"
	// FilesystemOverlay is overlayfs. A container root is usually on it.
	FilesystemOverlay = "overlay"
)
