package containerd

import (
	"errors"
	"fmt"
)

// ErrFailedReadingContent is used when there is an error reading from the content store.
var ErrFailedReadingContent = errors.New("failed reading from content store")

type errUnsupportedSnapshotter struct {
	name string
}

// Error returns the error message.
func (e errUnsupportedSnapshotter) Error() string {
	return fmt.Sprintf("snapshotter %s is not supported: snapshotters %s are supported", e.name, supportedSnapshotters)
}
