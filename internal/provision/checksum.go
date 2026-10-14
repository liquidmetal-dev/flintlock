package provision

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// DownloadVerified downloads url to destPath with the given permissions after
// checking its sha256 against the digest published at sumURL. The sum file
// has the sha256sum format: the hex digest, then whitespace and the file
// name. The download goes to a temporary file next to destPath and is moved
// into place only when the digest matches, so a mismatch never leaves a
// file at destPath.
func DownloadVerified(ctx context.Context, url, sumURL, destPath string, perm os.FileMode) error {
	want, err := fetchSHA256(ctx, sumURL)
	if err != nil {
		return err
	}

	body, err := get(ctx, url)
	if err != nil {
		return err
	}
	defer body.Close()

	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", destDir, err)
	}

	tmp, err := os.CreateTemp(destDir, "."+filepath.Base(destPath)+".*")
	if err != nil {
		return fmt.Errorf("creating temp file in %s: %w", destDir, err)
	}

	tmpPath := tmp.Name()

	defer func() {
		tmp.Close()
		os.Remove(tmpPath)
	}()

	digest := sha256.New()

	if _, err := io.Copy(io.MultiWriter(tmp, digest), body); err != nil {
		return fmt.Errorf("downloading %s: %w", url, err)
	}

	got := hex.EncodeToString(digest.Sum(nil))
	if got != want {
		return fmt.Errorf("%w for %s: got %s, want %s", ErrChecksumMismatch, url, got, want)
	}

	if err := tmp.Chmod(perm); err != nil {
		return fmt.Errorf("setting permissions on %s: %w", tmpPath, err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("writing %s: %w", tmpPath, err)
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		return fmt.Errorf("moving %s to %s: %w", tmpPath, destPath, err)
	}

	return nil
}

// ParseSHA256Sum returns the lower-case hex digest from the first field of
// a sha256sum style file ("<digest>  <name>").
func ParseSHA256Sum(content string) (string, error) {
	fields := strings.Fields(content)
	if len(fields) == 0 {
		return "", errEmptySHA256
	}

	digest := strings.ToLower(fields[0])
	if len(digest) != sha256.Size*2 {
		return "", fmt.Errorf("sha256 file field %q is not a sha256 digest", fields[0])
	}

	if _, err := hex.DecodeString(digest); err != nil {
		return "", fmt.Errorf("sha256 file field %q is not a sha256 digest: %w", fields[0], err)
	}

	return digest, nil
}

func fetchSHA256(ctx context.Context, sumURL string) (string, error) {
	body, err := get(ctx, sumURL)
	if err != nil {
		return "", err
	}
	defer body.Close()

	// A sha256sum line is under 100 bytes. The limit keeps a wrong URL from
	// being read in full.
	content, err := io.ReadAll(io.LimitReader(body, 4096))
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", sumURL, err)
	}

	digest, err := ParseSHA256Sum(string(content))
	if err != nil {
		return "", fmt.Errorf("parsing %s: %w", sumURL, err)
	}

	return digest, nil
}
