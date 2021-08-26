package core

import (
	"errors"
	"fmt"
)

var (
	ErrSpecRequired      = errors.New("microvm spec is required")
	ErrVMIDRequired      = errors.New("id for microvm is required")
	ErrNameRequired      = errors.New("name is required")
	ErrNamespaceRequired = errors.New("namespace is required")
)

// ErrTopicNotFound is an error created when a topic with a specific name isn't found.
type ErrTopicNotFound struct {
	Name string
}

// Error returns the error message.
func (e ErrTopicNotFound) Error() string {
	return fmt.Sprintf("topic %s not found", e.Name)
}

type ErrIncorrectVMIDFormat struct {
	ActualID string
}

// Error returns the error message.
func (e ErrIncorrectVMIDFormat) Error() string {
	return fmt.Sprintf("unexpected vmid format: %s", e.ActualID)
}
