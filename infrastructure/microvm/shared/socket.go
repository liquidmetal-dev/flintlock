package shared

import (
	"fmt"

	"github.com/spf13/afero"
)

// RemoveStaleSockets removes any of the given socket files that exist, so a VMM can bind to
// them on (re)create.
func RemoveStaleSockets(fs afero.Fs, paths ...string) error {
	for _, path := range paths {
		exists, err := afero.Exists(fs, path)
		if err != nil {
			return fmt.Errorf("checking if socket %s exists: %w", path, err)
		}

		if !exists {
			continue
		}

		if err := fs.Remove(path); err != nil {
			return fmt.Errorf("deleting existing socket %s: %w", path, err)
		}
	}

	return nil
}

// ResolveSocketPath returns the socket path for a process that is already running. That's
// path, unless only legacyPath exists, which means the process was started before sockets
// moved out of the state directory.
// Legacy fallback for #1226, to be removed in the next minor release.
func ResolveSocketPath(fs afero.Fs, path, legacyPath string) string {
	if exists, _ := afero.Exists(fs, path); exists {
		return path
	}

	if exists, _ := afero.Exists(fs, legacyPath); exists {
		return legacyPath
	}

	return path
}
