package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/liquidmetal-dev/flintlock/core/models"
)

// ValidateSocketDir checks that the socket directory is absolute and short enough that every
// per-microvm socket path stays within the unix socket path length limit.
func ValidateSocketDir(dir string) error {
	if !filepath.IsAbs(dir) {
		return newSocketDirError(dir, "must be an absolute path")
	}

	if maxLen := models.MaxSocketDirLength(); len(dir) > maxLen {
		return newSocketDirError(dir, fmt.Sprintf("is %d characters, the maximum is %d", len(dir), maxLen))
	}

	return nil
}

// Validate will validate the TLS config.
func (t TLSConfig) Validate() error {
	if t.Insecure {
		if isSet(t.KeyFile) || isSet(t.CertFile) {
			return errNoCertWhenInsecure
		}

		return nil
	}

	if isNotSet(t.CertFile) {
		return errCertRequired
	}

	if isNotSet(t.KeyFile) {
		return errKeyRequired
	}

	if t.ValidateClient && isNotSet(t.ClientCAFile) {
		return errClientCARequired
	}

	if _, err := os.Stat(t.CertFile); errors.Is(err, os.ErrNotExist) {
		return newCertMissingError("certificate file", t.CertFile)
	}

	if _, err := os.Stat(t.KeyFile); errors.Is(err, os.ErrNotExist) {
		return newCertMissingError("key file", t.KeyFile)
	}

	if t.ValidateClient {
		if _, err := os.Stat(t.ClientCAFile); errors.Is(err, os.ErrNotExist) {
			return newCertMissingError("client CA file", t.ClientCAFile)
		}
	}

	return nil
}

func isSet(val string) bool {
	return val != ""
}

func isNotSet(val string) bool {
	return !isSet(val)
}
