package provision

import (
	"context"
	"fmt"
	"path/filepath"
)

// InstallCloudHypervisor downloads and installs the given (or latest)
// version of cloud-hypervisor for the given normalised (amd64/arm64)
// architecture to InstallPath.
func InstallCloudHypervisor(ctx context.Context, runner *Runner, version, normalisedArch string) error {
	tag, err := ResolveTag(ctx, CloudHypervisorRepo, version)
	if err != nil {
		return fmt.Errorf("resolving cloud-hypervisor version: %w", err)
	}

	unameArch, err := UnameArch(normalisedArch)
	if err != nil {
		return err
	}

	binName := CloudHypervisorBin
	if unameArch == unameARM64 {
		binName = CloudHypervisorBin + "-" + unameARM64
	}

	url := DownloadURL(CloudHypervisorRepo, tag, binName)
	dest := filepath.Join(InstallPath, CloudHypervisorBin)

	if err := DownloadFile(ctx, url, dest, 0o755); err != nil {
		return fmt.Errorf("installing cloud-hypervisor %s: %w", tag, err)
	}

	if err := runner.Run(dest, "--version"); err != nil {
		return fmt.Errorf("cloud-hypervisor %s did not install correctly: %w", tag, err)
	}

	return nil
}
