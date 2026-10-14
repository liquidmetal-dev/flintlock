package provision

import "errors"

var (
	// ErrChecksumMismatch is returned when a downloaded file does not match
	// the sha256 published next to it. The file is not installed.
	ErrChecksumMismatch = errors.New("sha256 mismatch")

	errEmptySHA256 = errors.New("sha256 file is empty")
)
