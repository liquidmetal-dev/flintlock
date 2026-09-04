package provision

import (
	"github.com/spf13/cobra"

	"github.com/liquidmetal-dev/flintlock/internal/provision"
)

func newAptCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "apt",
		Short: "Install all apt packages required by flintlock",
		RunE: func(cmd *cobra.Command, _ []string) error {
			runner := provision.NewRunner(cmd.OutOrStdout(), cmd.ErrOrStderr())

			return provision.InstallAptPackages(runner)
		},
	}
}
