package provision

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/liquidmetal-dev/flintlock/internal/provision"
)

func newCloudHypervisorCommand() *cobra.Command {
	var version string

	cmd := &cobra.Command{
		Use:   "cloudhypervisor",
		Short: "Install Cloud Hypervisor",
		RunE: func(cmd *cobra.Command, _ []string) error {
			arch, err := hostArch()
			if err != nil {
				return err
			}

			runner := provision.NewRunner(cmd.OutOrStdout(), cmd.ErrOrStderr())

			fmt.Fprintf(cmd.OutOrStdout(), "Installing Cloud Hypervisor version %s to %s\n", version, provision.InstallPath)

			return provision.InstallCloudHypervisor(cmd.Context(), runner, version, arch)
		},
	}

	cmd.Flags().StringVarP(&version, "version", "v", provision.VersionFromEnv(provision.CloudHypervisorVersionEnv), "Version to install")

	return cmd
}
