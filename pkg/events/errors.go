package events

import "fmt"

// ErrTopicNotFound is an error created when a topic with a specific name isn't found.
type ErrTopicNotFound struct {
	Name string
}

// Error returns the error message.
func (e ErrTopicNotFound) Error() string {
	return fmt.Sprintf("topic %s not found", e.Name)
}
