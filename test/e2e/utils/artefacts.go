//go:build e2e
// +build e2e

package utils

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	// The tests run as root, and the files are read by a user who is not.
	artefactDirPerm  = 0o755
	artefactFilePerm = 0o644

	// diskImageExtension is the extension of the disks which a provider keeps
	// in the state directory of a microvm, such as the one with the cloud-init
	// data.
	diskImageExtension = ".img"

	// The state directory of a microvm ends with its namespace, its name and
	// its uid.
	stateDirElements = 3
)

// The functions in this file return an error and do not fail the test, so
// that they can be used when the test has failed already.

// ArtefactsPath returns the directory which the files of the state directory
// of a microvm are saved to. It is in the directory of the test, and has the
// namespace, the name and the uid of the microvm.
func ArtefactsPath(artefactsDir, testName, stateDir string) string {
	elements := []string{}

	for _, element := range strings.Split(filepath.Clean(stateDir), string(filepath.Separator)) {
		if element != "" {
			elements = append(elements, element)
		}
	}

	if len(elements) > stateDirElements {
		elements = elements[len(elements)-stateDirElements:]
	}

	return filepath.Join(append([]string{artefactsDir, testName}, elements...)...)
}

// SaveStateFiles copies the files of the state directory of a microvm to
// another directory, and returns the names of the ones which it has saved and
// of the ones which it has skipped. flintlockd removes the state directory
// when the microvm is deleted.
//
// The names of the files are not listed here, so that a file which a provider
// did not have before is saved as well. What is not a regular file is skipped,
// and so are the disk images.
//
// A microvm which flintlockd has not started to create does not have a state
// directory, which is not an error. A file which cannot be saved does not stop
// the others.
func SaveStateFiles(stateDir, destDir string) ([]string, []string, error) {
	saved := []string{}
	skipped := []string{}

	entries, err := os.ReadDir(stateDir)
	if err != nil {
		if os.IsNotExist(err) {
			return saved, skipped, nil
		}

		return saved, skipped, fmt.Errorf("reading the state directory: %w", err)
	}

	var failed []error

	for _, entry := range entries {
		name := entry.Name()

		if !entry.Type().IsRegular() || filepath.Ext(name) == diskImageExtension {
			skipped = append(skipped, name)

			continue
		}

		if err := copyFile(filepath.Join(stateDir, name), destDir, name); err != nil {
			failed = append(failed, err)

			continue
		}

		saved = append(saved, name)
	}

	return saved, skipped, errors.Join(failed...)
}

func copyFile(source, destDir, name string) error {
	from, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("opening %s: %w", source, err)
	}

	defer from.Close()

	if err := makeReadableDirectories(destDir); err != nil {
		return fmt.Errorf("creating the directory for %s: %w", name, err)
	}

	dest := filepath.Join(destDir, name)

	to, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, artefactFilePerm)
	if err != nil {
		return fmt.Errorf("creating %s: %w", dest, err)
	}

	if _, err := io.Copy(to, from); err != nil {
		to.Close()

		return fmt.Errorf("copying %s to %s: %w", source, dest, err)
	}

	if err := to.Close(); err != nil {
		return fmt.Errorf("closing %s: %w", dest, err)
	}

	// The mode of a file which was there is not changed when it is opened, and
	// the mode of a new one depends on the umask.
	if err := os.Chmod(dest, artefactFilePerm); err != nil {
		return fmt.Errorf("changing the mode of %s: %w", dest, err)
	}

	return nil
}

// makeReadableDirectories creates a directory and the ones above it which do
// not exist. The mode of a new directory depends on the umask, so it is set
// for each of the directories which are created.
func makeReadableDirectories(dir string) error {
	created := []string{}

	for current := filepath.Clean(dir); ; current = filepath.Dir(current) {
		if _, err := os.Stat(current); err == nil || current == filepath.Dir(current) {
			break
		}

		created = append(created, current)
	}

	if err := os.MkdirAll(dir, artefactDirPerm); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}

	for _, current := range created {
		if err := os.Chmod(current, artefactDirPerm); err != nil {
			return fmt.Errorf("changing the mode of %s: %w", current, err)
		}
	}

	return nil
}
