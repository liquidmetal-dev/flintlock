package provision

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/liquidmetal-dev/flintlock/internal/provision"
)

func newDevPoolCommand() *cobra.Command {
	var thinpool string

	cmd := &cobra.Command{
		Use:   "devpool",
		Short: "Set up a loop device thinpool (development environments)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			runner := provision.NewRunner(cmd.OutOrStdout(), cmd.ErrOrStderr())

			paths := provision.BuildContainerdPaths(true)
			if err := provision.MakeContainerdDirs(paths); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Will create loop-back thinpool %s-thinpool\n", thinpool)

			if err := provision.AllDevPool(runner, paths, thinpool); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "remember to set pool_name to %s-thinpool in your containerd config\n", thinpool)

			return nil
		},
	}

	cmd.Flags().StringVarP(&thinpool, "thinpool", "t", provision.DefaultDevThinpool, "Name of thinpool to create")

	return cmd
}
