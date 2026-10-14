package provision

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/liquidmetal-dev/flintlock/internal/provision"
	"github.com/liquidmetal-dev/flintlock/internal/version"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
)

func newErofsUtilsCommand() *cobra.Command {
	var release string

	cmd := &cobra.Command{
		Use:   "erofs-utils",
		Short: "Install the pinned mkfs.erofs used to build overlay base images",
		Long: "erofs-utils downloads the static mkfs.erofs " + defaults.ErofsUtilsVersion +
			" attached to a flintlock release, checks it against the published sha256 and " +
			"installs it at " + defaults.MkfsErofsPath + ". A system mkfs.erofs is not replaced.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			arch, err := hostArch()
			if err != nil {
				return err
			}

			runner := provision.NewRunner(cmd.OutOrStdout(), cmd.ErrOrStderr())

			fmt.Fprintf(cmd.OutOrStdout(), "Installing mkfs.erofs %s from flintlock release %s to %s\n",
				defaults.ErofsUtilsVersion, release, defaults.MkfsErofsPath)

			return provision.InstallErofsUtils(cmd.Context(), runner, release, arch)
		},
	}

	cmd.Flags().StringVarP(&release, "version", "v", defaultErofsUtilsRelease(),
		"Flintlock release that carries the asset ($"+provision.FlintlockVersionEnv+
			", else this binary's own release tag, else latest)")

	return cmd
}

// defaultErofsUtilsRelease is the flintlock release the asset is fetched
// from when --version is not given. The release of this binary is preferred
// over latest because the pinned erofs-utils version is compiled in: the
// asset of the same release always carries that version.
func defaultErofsUtilsRelease() string {
	if v := os.Getenv(provision.FlintlockVersionEnv); v != "" {
		return v
	}

	if strings.HasPrefix(version.Version, "v") {
		return version.Version
	}

	return provision.DefaultVersion
}
