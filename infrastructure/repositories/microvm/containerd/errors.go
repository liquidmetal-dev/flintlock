package containerd

import (
	"errors"
	"fmt"
)

// ErrFailedReadingContent is used when there is an error reading from the content store.
var ErrFailedReadingContent = errors.New("failed reading from content store")

type errSpecNotFound struct {
	name      string
	namespace string
}

// Error returns the error message.
func (e errSpecNotFound) Error() string {
	return fmt.Sprintf("microvm spec %s/%s not found", e.namespace, e.name)
}

// IsSpecNotFound tests an error to see if its a spec not found error.
func IsSpecNotFound(err error) bool {
	var e errSpecNotFound

	return errors.Is(err, e)
}
