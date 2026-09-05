package provision

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// InstallFirecracker downloads and installs the given (or latest) version of
// firecracker for the given normalised (amd64/arm64) architecture to InstallPath.
func InstallFirecracker(ctx context.Context, runner *Runner, version, normalisedArch string) error {
	tag, err := ResolveTag(ctx, FirecrackerRepo, version)
	if err != nil {
		return fmt.Errorf("resolving firecracker version: %w", err)
	}

	// firecracker's own release artefacts use uname -m style naming
	// (x86_64/aarch64), not the amd64/arm64 naming used elsewhere.
	arch, err := UnameArch(normalisedArch)
	if err != nil {
		return err
	}

	binName := fmt.Sprintf("%s-%s-%s.tgz", FirecrackerBin, tag, arch)
	url := DownloadURL(FirecrackerRepo, tag, binName)

	tempDir, err := os.MkdirTemp("", "flintlock-provision-firecracker")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	if err := ExtractTarGz(ctx, url, tempDir); err != nil {
		return fmt.Errorf("installing firecracker %s: %w", tag, err)
	}

	releaseDir := "release-" + tag + "-" + arch
	binFile := "firecracker-" + tag + "-" + arch
	downloaded := filepath.Join(tempDir, releaseDir, binFile)
	dest := filepath.Join(InstallPath, FirecrackerBin)

	if err := copyExecutable(downloaded, dest); err != nil {
		return fmt.Errorf("installing firecracker %s: %w", tag, err)
	}

	if err := runner.Run(dest, "--version"); err != nil {
		return fmt.Errorf("firecracker %s did not install correctly: %w", tag, err)
	}

	return nil
}

func copyExecutable(src, dest string) error {
	content, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("reading %s: %w", src, err)
	}

	if err := os.WriteFile(dest, content, 0o755); err != nil { //nolint:gosec // binaries must be executable
		return fmt.Errorf("writing %s: %w", dest, err)
	}

	return nil
}
