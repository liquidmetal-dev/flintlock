package provision

import (
	"context"
	"fmt"
	"strings"

	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
)

// ErofsUtilsAssetName returns the name of the mkfs.erofs release asset for
// the given erofs-utils version and normalised (amd64/arm64) architecture.
// hack/scripts/build_erofs_utils.sh produces the asset under this name and
// the release workflow attaches it to the flintlock release together with
// a "<name>.sha256" file.
func ErofsUtilsAssetName(erofsVersion, arch string) string {
	return fmt.Sprintf("mkfs.erofs-%s-linux-%s", erofsVersion, arch)
}

// InstallErofsUtils downloads the pinned mkfs.erofs (defaults.ErofsUtilsVersion)
// attached to the given (or latest) flintlock release for arch, verifies it
// against the published sha256 and installs it at defaults.MkfsErofsPath.
// A system mkfs.erofs is left alone. The installed binary must report the
// pinned version.
func InstallErofsUtils(ctx context.Context, runner *Runner, version, arch string) error {
	tag, err := ResolveTag(ctx, FlintlockRepo, version)
	if err != nil {
		return fmt.Errorf("resolving flintlock version: %w", err)
	}

	asset := ErofsUtilsAssetName(defaults.ErofsUtilsVersion, arch)
	url := DownloadURL(FlintlockRepo, tag, asset)
	dest := defaults.MkfsErofsPath

	if err := DownloadVerified(ctx, url, url+".sha256", dest, 0o755); err != nil {
		return fmt.Errorf("installing mkfs.erofs %s from flintlock %s: %w", defaults.ErofsUtilsVersion, tag, err)
	}

	return checkMkfsErofsVersion(runner, dest, defaults.ErofsUtilsVersion)
}

// checkMkfsErofsVersion runs "<bin> -V" and checks that the first line
// names the expected version. mkfs.erofs 1.9.x prints
// "mkfs.erofs (erofs-utils) <version>" on that line.
func checkMkfsErofsVersion(runner *Runner, bin, want string) error {
	out, err := runner.Output(bin, "-V")
	if err != nil {
		return fmt.Errorf("mkfs.erofs did not install correctly: %w", err)
	}

	first, _, _ := strings.Cut(out, "\n")

	fields := strings.Fields(first)
	if len(fields) == 0 || fields[len(fields)-1] != want {
		return fmt.Errorf("%s reports %q, expected version %s", bin, first, want)
	}

	return nil
}
