package shared

import (
	"fmt"
	"os"
	"path/filepath"

	securejoin "github.com/cyphar/filepath-securejoin"

	"github.com/liquidmetal-dev/flintlock/core/errors"
)

// ResolveImageFile returns the host path of filename within the image mounted at
// root, which must be an absolute path. Symlinks are followed as if root were the
// filesystem root, so the result never escapes the mount. The result must be a
// regular file.
func ResolveImageFile(root, filename string) (string, error) {
	// An empty or relative root would resolve against the daemon's working directory.
	if !filepath.IsAbs(root) {
		return "", fmt.Errorf("%w: image mount path %q is not absolute", errors.ErrInvalidImageFilePath, root)
	}

	if !filepath.IsLocal(filename) {
		return "", fmt.Errorf("%w: %q is not a relative path within the image", errors.ErrInvalidImageFilePath, filename)
	}

	path, err := securejoin.SecureJoin(filepath.Clean(root), filename)
	if err != nil {
		return "", fmt.Errorf("%w: resolving %q: %w", errors.ErrInvalidImageFilePath, filename, err)
	}

	info, err := os.Lstat(path)
	if err != nil {
		return "", fmt.Errorf("%w: %q: %w", errors.ErrInvalidImageFilePath, filename, err)
	}

	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%w: %q is not a regular file", errors.ErrInvalidImageFilePath, filename)
	}

	return path, nil
}
