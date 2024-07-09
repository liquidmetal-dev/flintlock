package application

import (
	"errors"
	"fmt"

	"github.com/liquidmetal-dev/flintlock/core/models"
)

var errUIDRequired = errors.New("uid is required")

type specAlreadyExistsError struct {
	name      string
	namespace string
	uid       string
}

// Error returns the error message.
func (e specAlreadyExistsError) Error() string {
	return fmt.Sprintf("microvm spec %s/%s/%s already exists", e.namespace, e.name, e.uid)
}

type specNotFoundError struct {
	name      string
	namespace string
	uid       string
}

// Error returns the error message.
func (e specNotFoundError) Error() string {
	if e.name != "" {
		return fmt.Sprintf("microvm spec %s/%s/%s not found", e.namespace, e.name, e.uid)
	}

	return fmt.Sprintf("microvm spec %s not found", e.uid)
}

type reachedMaximumRetryError struct {
	vmid    models.VMID
	retries int
}

func (e reachedMaximumRetryError) Error() string {
	return fmt.Sprintf(
		"microvm reconciliation for %s failed %d times",
		e.vmid.String(),
		e.retries,
	)
}
