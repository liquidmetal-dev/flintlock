package application

import (
	"errors"
	"fmt"
)

var (
	errIDRequired    = errors.New("microvm id is required")
	errNotImplemeted = errors.New("not implemented")
)

type errSpecAlreadyExists struct {
	name      string
	namespace string
}

// Error returns the error message.
func (e errSpecAlreadyExists) Error() string {
	return fmt.Sprintf("microvm spec %s/%s already exists", e.namespace, e.name)
}

type errSpecNotFound struct {
	name      string
	namespace string
}

// Error returns the error message.
func (e errSpecNotFound) Error() string {
	return fmt.Sprintf("microvm spec %s/%s not found", e.namespace, e.name)
}
