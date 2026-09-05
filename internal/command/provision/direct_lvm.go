package provision

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/liquidmetal-dev/flintlock/internal/provision"
)

func newDirectLVMCommand() *cobra.Command {
	var (
		unattended bool
		disk       string
		thinpool   string
		skipApt    bool
	)

	cmd := &cobra.Command{
		Use:   "direct-lvm",
		Short: "Set up a direct-lvm thinpool",
		RunE: func(cmd *cobra.Command, _ []string) error {
			runner := provision.NewRunner(cmd.OutOrStdout(), cmd.ErrOrStderr())

			if !skipApt {
				if err := provision.InstallAptPackages(runner); err != nil {
					return err
				}
			}

			diskName := disk
			if diskName == "" {
				fmt.Fprintln(cmd.ErrOrStderr(), "WARNING: --disk has not been set. If you continue, "+
					"flintlock-provision will attempt to detect a free disk for formatting. Any data will be lost.")

				if !unattended && !provision.Confirm(cmd.InOrStdin(), cmd.OutOrStdout(), "Are you sure you wish to continue? (y/n) ") {
					return errAborted
				}

				found, err := provision.FindFreeDisk(runner)
				if err != nil {
					return err
				}

				diskName = found
			}

			diskPath := filepath.Join("/dev", filepath.Base(diskName))

			fmt.Fprintf(cmd.OutOrStdout(), "Will use %s for direct-lvm thinpool %s\n", diskPath, thinpool)
			fmt.Fprintf(cmd.ErrOrStderr(), "WARNING: all existing data on %s will be overwritten.\n", diskPath)

			if !unattended && !provision.Confirm(cmd.InOrStdin(), cmd.OutOrStdout(), "") {
				return errAborted
			}

			if err := provision.AllDirectLVM(runner, diskPath, thinpool); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "remember to set pool_name to %s-thinpool in your containerd config\n", thinpool)

			return nil
		},
	}

	cmd.Flags().BoolVarP(&unattended, "unattended", "y", false, "Autoapprove all prompts (danger)")
	cmd.Flags().StringVarP(&thinpool, "thinpool", "t", provision.DefaultThinpool, "Name of thinpool to create")
	cmd.Flags().StringVarP(&disk, "disk", "d", "",
		"Name of a blank, unpartitioned disk to use for the direct-lvm thinpool")
	cmd.Flags().BoolVarP(&skipApt, "skip-apt", "s", false, "Skip installation of apt packages")

	return cmd
}
