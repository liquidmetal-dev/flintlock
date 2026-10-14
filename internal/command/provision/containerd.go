package provision

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/liquidmetal-dev/flintlock/internal/provision"
)

func newContainerdCommand() *cobra.Command {
	var (
		version      string
		thinpool     string
		dev          bool
		skipThinpool bool
	)

	cmd := &cobra.Command{
		Use:   "containerd",
		Short: "Install, configure and start containerd",
		RunE: func(cmd *cobra.Command, _ []string) error {
			arch, err := hostArch()
			if err != nil {
				return err
			}

			if dev && !cmd.Flags().Changed("thinpool") {
				thinpool = provision.DefaultDevThinpool
			}

			if skipThinpool {
				thinpool = ""
			}

			runner := provision.NewRunner(cmd.OutOrStdout(), cmd.ErrOrStderr())

			fmt.Fprintf(cmd.OutOrStdout(), "Installing containerd version %s to %s\n", version, provision.InstallPath)

			_, err = provision.AllContainerd(cmd.Context(), runner, version, thinpool, arch, dev)

			return err
		},
	}

	cmd.Flags().StringVarP(&version, "version", "v", provision.VersionFromEnv(provision.ContainerdVersionEnv), "Version to install")
	cmd.Flags().StringVarP(&thinpool, "thinpool", "t", provision.DefaultThinpool,
		"Name of thinpool to include in config toml")
	cmd.Flags().BoolVar(&dev, "dev", false,
		"Set up development environment. Containerd will keep state under 'dev' tagged paths.")
	cmd.Flags().BoolVar(&skipThinpool, "skip-thinpool", false,
		"Leave the devmapper snapshotter out of the config toml. Use on a host with no thin pool.")

	return cmd
}
