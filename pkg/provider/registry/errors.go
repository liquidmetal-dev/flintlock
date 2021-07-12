package registry

import "errors"

var (
	// ErrDuplicatePlugin is an error for when plugin registration has been called multiple
	// time for a plugin with the same name.
	ErrDuplicatePlugin = errors.New("plugin with the same name has already been registered")

	// ErrPluginNotFound is an error when a plugin isn't found with a supplied name.
	ErrPluginNotFound = errors.New("plugin not found")
)
