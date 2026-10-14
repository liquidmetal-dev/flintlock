package overlay

import (
	"errors"
	"fmt"
	"strings"

	"github.com/opencontainers/go-digest"
)

var (
	errNoLayers       = errors.New("image has no layers")
	errBaseNotInIndex = errors.New("base is not in the index")
)

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
