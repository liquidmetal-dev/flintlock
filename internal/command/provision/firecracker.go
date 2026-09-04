package provision

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/liquidmetal-dev/flintlock/internal/provision"
)

func newFirecrackerCommand() *cobra.Command {
	var version string

	cmd := &cobra.Command{
		Use:   "firecracker",
		Short: "Install firecracker",
		RunE: func(cmd *cobra.Command, _ []string) error {
			arch, err := hostArch()
			if err != nil {
				return err
			}

			runner := provision.NewRunner(cmd.OutOrStdout(), cmd.ErrOrStderr())

			fmt.Fprintf(cmd.OutOrStdout(), "Installing firecracker version %s to %s\n", version, provision.InstallPath)

			return provision.InstallFirecracker(cmd.Context(), runner, version, arch)
		},
	}

	cmd.Flags().StringVarP(&version, "version", "v", provision.DefaultVersion, "Version to install")

	return cmd
}
