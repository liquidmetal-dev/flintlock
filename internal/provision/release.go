package provision

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// userAgent is sent on every outbound HTTP request this package makes.
const userAgent = "flintlock-provision"

// release is the subset of the GitHub releases API response used to find
// the tag of the latest release of a repository.
type release struct {
	TagName string `json:"tag_name"`
}

// LatestReleaseTag returns the tag of the latest release of the given
// "owner/repo" GitHub repository.
func LatestReleaseTag(ctx context.Context, repo string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("building request for %s: %w", url, err)
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching latest release for %s: %w", repo, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetching latest release for %s: unexpected status %s", repo, resp.Status)
	}

	rel := release{}
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", fmt.Errorf("decoding latest release for %s: %w", repo, err)
	}

	if rel.TagName == "" {
		return "", fmt.Errorf("no tag_name found in latest release for %s", repo)
	}

	return rel.TagName, nil
}

// VersionFromEnv returns the value of envVar if set, otherwise DefaultVersion.
// It matches provision.sh's use of e.g. FIRECRACKER_VERSION="${FIRECRACKER:=$DEFAULT_VERSION}"
// to let a component's default version be overridden via an environment variable.
func VersionFromEnv(envVar string) string {
	if v := os.Getenv(envVar); v != "" {
		return v
	}

	return DefaultVersion
}

// ResolveTag returns tag as-is unless it is DefaultVersion, in which case
// the latest release tag for repo is looked up and returned instead.
func ResolveTag(ctx context.Context, repo, tag string) (string, error) {
	if tag != DefaultVersion {
		return tag, nil
	}

	return LatestReleaseTag(ctx, repo)
}

// DownloadURL returns the URL of a binary attached to a repository's release.
func DownloadURL(repo, tag, bin string) string {
	return fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, tag, bin)
}

// RawURL returns the URL of a file at the root of a repository's default branch.
func RawURL(repo, fileName string) string {
	return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s", repo, DefaultBranch, fileName)
}

// ContainerdReleaseBinName returns the name of the containerd release
// tarball for the given tag and architecture.
func ContainerdReleaseBinName(tag, arch string) string {
	version := strings.TrimPrefix(tag, "v")

	return fmt.Sprintf("containerd-%s-linux-%s.tar.gz", version, arch)
}

// DownloadFile downloads url and writes it to destPath with the given
// permissions, replacing the script's "wget -O" based install_release_bin.
func DownloadFile(ctx context.Context, url, destPath string, perm os.FileMode) error {
	body, err := get(ctx, url)
	if err != nil {
		return err
	}
	defer body.Close()

	out, err := os.OpenFile(destPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
	if err != nil {
		return fmt.Errorf("creating %s: %w", destPath, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, body); err != nil {
		return fmt.Errorf("downloading %s to %s: %w", url, destPath, err)
	}

	return nil
}

// ExtractTarGz downloads the gzipped tarball at url and extracts it into destDir,
// replacing the script's "curl | tar xz" based install_release_tar.
func ExtractTarGz(ctx context.Context, url, destDir string) error {
	body, err := get(ctx, url)
	if err != nil {
		return err
	}
	defer body.Close()

	gzr, err := gzip.NewReader(body)
	if err != nil {
		return fmt.Errorf("reading gzip stream from %s: %w", url, err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			return nil
		}

		if err != nil {
			return fmt.Errorf("reading tar stream from %s: %w", url, err)
		}

		if err := extractTarEntry(tr, header, destDir); err != nil {
			return err
		}
	}
}

func extractTarEntry(tr *tar.Reader, header *tar.Header, destDir string) error {
	dest, err := safeJoin(destDir, header.Name)
	if err != nil {
		return fmt.Errorf("extracting tar entry %q: %w", header.Name, err)
	}

	switch header.Typeflag {
	case tar.TypeDir:
		if err := os.MkdirAll(dest, os.FileMode(header.Mode)); err != nil {
			return fmt.Errorf("creating directory %s: %w", dest, err)
		}
	case tar.TypeReg:
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return fmt.Errorf("creating directory %s: %w", filepath.Dir(dest), err)
		}

		out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode))
		if err != nil {
			return fmt.Errorf("creating file %s: %w", dest, err)
		}
		defer out.Close()

		// #nosec G110 -- bounded by the size of a GitHub release tarball we control the URL of.
		if _, err := io.Copy(out, tr); err != nil {
			return fmt.Errorf("writing file %s: %w", dest, err)
		}
	}

	return nil
}

// safeJoin joins destDir and name, returning an error if the result would
// escape destDir (e.g. via a ".." or absolute path in name). This guards
// extractTarEntry against maliciously or accidentally crafted tar entries
// (a "Zip Slip" style path traversal).
func safeJoin(destDir, name string) (string, error) {
	cleanDestDir := filepath.Clean(destDir)
	dest := filepath.Join(cleanDestDir, name)

	if dest != cleanDestDir && !strings.HasPrefix(dest, cleanDestDir+string(os.PathSeparator)) {
		return "", fmt.Errorf("%q escapes destination directory %q", name, destDir)
	}

	return dest, nil
}

func get(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building request for %s: %w", url, err)
	}

	req.Header.Set("User-Agent", userAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", url, err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()

		return nil, fmt.Errorf("fetching %s: unexpected status %s", url, resp.Status)
	}

	return resp.Body, nil
}
