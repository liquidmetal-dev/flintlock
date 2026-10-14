package provision

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/liquidmetal-dev/flintlock/internal/provision"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
)

func newStateDirCommand() *cobra.Command {
	var (
		opts       provision.StateDirOptions
		unattended bool
	)

	cmd := &cobra.Command{
		Use:   "state-dir",
		Short: "Prepare the flintlockd state directory on a filesystem with reflink support",
		Long: "state-dir checks that the flintlockd state directory can clone files with FICLONE, " +
			"which overlay boot needs. With no --device or --loop-file it only verifies the directory. " +
			"With --device it formats the device as XFS with reflink=1, mounts it at the state directory " +
			"and persists the mount in /etc/fstab. With --loop-file it does the same with a sparse file " +
			"on a loop device, for development and e2e hosts.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			runner := provision.NewRunner(cmd.OutOrStdout(), cmd.ErrOrStderr())

			target := opts.Device
			if target == "" {
				target = opts.LoopFile
			}

			if target != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "WARNING: %s will be formatted and mounted at %s.\n", target, opts.Dir)

				if !unattended && !provision.Confirm(cmd.InOrStdin(), cmd.OutOrStdout(), "") {
					return errAborted
				}
			}

			return provision.NewStateDir(runner, cmd.OutOrStdout()).Setup(opts)
		},
	}

	cmd.Flags().StringVar(&opts.Dir, "dir", defaults.StateRootDir, "The flintlockd state directory")
	cmd.Flags().StringVar(&opts.Device, "device", "",
		"Block device or logical volume to format as XFS and mount at the state directory")
	cmd.Flags().StringVar(&opts.LoopFile, "loop-file", "",
		"Sparse file to format as XFS and mount at the state directory through a loop device")
	cmd.Flags().IntVar(&opts.SizeGB, "size", provision.DefaultStateDirLoopSizeGB,
		"Size in GB of the sparse file created for --loop-file")
	cmd.Flags().BoolVar(&opts.Force, "force", false,
		"Format the device or loop file even when it already holds a filesystem")
	cmd.Flags().BoolVarP(&unattended, "unattended", "y", false, "Autoapprove all prompts (danger)")

	return cmd
}
