package overlay

import (
	"errors"
	"fmt"
	"strings"

	"github.com/opencontainers/go-digest"
)

var (
	errNoLayers        = errors.New("image has no layers")
	errBaseNotInIndex  = errors.New("base is not in the index")
	errDiffIDNotSHA256 = errors.New("diff ID is not a sha256 digest")
)

// DiffIDError reports a layer whose decompressed tar stream does not hash
// to the diff ID its source claimed. The chain ID, and so the base key,
// is derived from the claimed diff IDs, so a mismatch would record a base
// under a key that does not identify its bytes. The build fails instead.
type DiffIDError struct {
	// LayerIndex is the position of the layer, lowest layer first from 0.
	LayerIndex int
	// Want is the diff ID the source claimed.
	Want digest.Digest
	// Got is the digest of the stream the source gave.
	Got digest.Digest
}

func (e *DiffIDError) Error() string {
	return fmt.Sprintf("layer %d: stream hashes to %s, diff ID is %s", e.LayerIndex, e.Got, e.Want)
}

// VersionError reports a mkfs.erofs whose version is not the one the
// profile fixes.
type VersionError struct {
	// Path is the mkfs.erofs that was run.
	Path string
	// Reported is the version line mkfs.erofs printed.
	Reported string
	// Want is the version the profile requires.
	Want string
}

func (e *VersionError) Error() string {
	return fmt.Sprintf("%s reports %q, profile requires erofs-utils %s", e.Path, e.Reported, e.Want)
}

// MkfsError reports a mkfs.erofs run that exited with an error. Stderr
// holds what the command printed.
type MkfsError struct {
	// Args is the command line without the program path.
	Args []string
	// Stderr is the standard error of the command, trimmed.
	Stderr string
	// Err is the error exec returned.
	Err error
}

func (e *MkfsError) Error() string {
	msg := "mkfs.erofs " + strings.Join(e.Args, " ") + ": " + e.Err.Error()
	if e.Stderr != "" {
		msg += ": " + e.Stderr
	}

	return msg
}

func (e *MkfsError) Unwrap() error {
	return e.Err
}

// HardlinkError reports a layer entry that is a hardlink to a path the
// layer does not hold. The merge cannot link into a lower layer, so the
// build fails.
type HardlinkError struct {
	// LayerIndex is the position of the layer, lowest layer first from 0.
	LayerIndex int
	// DiffID is the diff ID of the layer.
	DiffID digest.Digest
	// Entry is the path of the hardlink entry in the layer.
	Entry string
	// Target is the path the entry links to.
	Target string
}

func (e *HardlinkError) Error() string {
	return fmt.Sprintf("layer %d (%s): entry %q is a hardlink to %q, which is not in the same layer",
		e.LayerIndex, e.DiffID, e.Entry, e.Target)
}

var (
	// ErrReflinkNotSupported reports a state directory whose filesystem
	// cannot clone files with FICLONE. It is a configuration error: there is
	// no fallback to a full copy, because that would hide a misprovisioned
	// host.
	ErrReflinkNotSupported = errors.New("filesystem does not support reflinks")
	// ErrDiskTooSmall reports a writable disk size under the minimum.
	ErrDiskTooSmall = errors.New("writable disk size is under the minimum")
)

// Ext4Error reports a mkfs.ext4 run that exited with an error. Stderr
// holds what the command printed.
type Ext4Error struct {
	// Args is the command line without the program path.
	Args []string
	// Stderr is the standard error of the command, trimmed.
	Stderr string
	// Err is the error exec returned.
	Err error
}

func (e *Ext4Error) Error() string {
	msg := "mkfs.ext4 " + strings.Join(e.Args, " ") + ": " + e.Err.Error()
	if e.Stderr != "" {
		msg += ": " + e.Stderr
	}

	return msg
}

func (e *Ext4Error) Unwrap() error {
	return e.Err
}
