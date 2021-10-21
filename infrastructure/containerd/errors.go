package containerd

import (
	"errors"
	"fmt"
)

// ErrReadingContent is used when there is an error reading from the content store.
var ErrReadingContent = errors.New("failed reading from content store")

type unsupportedSnapshotterError struct {
	name string
}

// Error returns the error message.
func (e unsupportedSnapshotterError) Error() string {
	return fmt.Sprintf("snapshotter %s is not supported: snapshotters %s are supported", e.name, supportedSnapshotters)
}
