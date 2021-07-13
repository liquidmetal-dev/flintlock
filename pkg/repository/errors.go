package repository

import "fmt"

type errSpecNotFound struct {
	name      string
	namespace string
}

func (e errSpecNotFound) Error() string {
	return fmt.Sprintf("microvm spec %s/%s not found", e.namespace, e.name)
}

func IsSpecNotFound(err error) bool {
	_, ok := err.(errSpecNotFound)
	return ok
}
