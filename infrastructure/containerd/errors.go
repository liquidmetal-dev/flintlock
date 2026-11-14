package containerd

import (
	"errors"
	"fmt"
)

var (
	// ErrReadingContent is used when there is an error reading from the content store.
	ErrReadingContent = errors.New("failed reading from content store")

	errLayerCount = errors.New("layer count does not match the image config")
	errNotALayer  = errors.New("manifest entry is not a layer")
)

type unsupportedSnapshotterError struct {
	name string
}

// Error returns the error message.
func (e unsupportedSnapshotterError) Error() string {
	return fmt.Sprintf("snapshotter %s is not supported: snapshotters %s are supported", e.name, supportedSnapshotters)
}
