package provision

import (
	"fmt"
	"os"
	"path/filepath"
)

// writeFileIfMissing writes content to path only if no file exists there yet.
func writeFileIfMissing(path, content string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", filepath.Dir(path), err)
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil { //nolint:gosec // not sensitive content
		return fmt.Errorf("writing %s: %w", path, err)
	}

	return nil
}
