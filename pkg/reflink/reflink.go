// Package reflink checks whether a directory is on a filesystem that can
// clone files with the FICLONE ioctl. The provisioner uses it to verify a
// state directory and the daemon uses it to probe the host at start.
package reflink

import "errors"

// ErrNotSupported is returned by Probe when the filesystem under the
// directory does not support FICLONE.
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
