package provision

import "errors"

var (
	// ErrChecksumMismatch is returned when a downloaded file does not match
	// the sha256 published next to it. The file is not installed.
	ErrChecksumMismatch = errors.New("sha256 mismatch")

	// ErrErofsUtilsArch is returned when the host is not linux/amd64, the
	// only architecture flintlock releases ship mkfs.erofs for. arm64 is
	// tracked in https://github.com/liquidmetal-dev/flintlock/issues/1317.
	ErrErofsUtilsArch = errors.New("flintlock releases ship mkfs.erofs for linux/amd64 only " +
		"(arm64 is tracked in https://github.com/liquidmetal-dev/flintlock/issues/1317)")

	// ErrNoReflink is returned when the state directory is on a filesystem
	// that cannot clone files with FICLONE and no device or loop file was
	// given to create one.
	ErrNoReflink = errors.New("state directory has no reflink support")

	errEmptySHA256 = errors.New("sha256 file is empty")
)
