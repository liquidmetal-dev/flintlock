package containerd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/weaveworks/reignite/pkg/defaults"
)

const (
	volSnapshotterFlagName    = "containerd-volume-ss"
	kernelSnapshotterFlagName = "containerd-kernel-ss"
	socketPathFlagName        = "containerd-socket"

	supportedSnapshotters = "overlayfs,native,devmapper"
)

// Config holds the contaierd configuration.
type Config struct {
	// SnapshotterKernel is the name of the containerd snapshotter to use for kernel images.
	SnapshotterKernel string
	// SnapshotterVolume is the name of the containerd snapshotter to use for volume (inc initrd) images.
	SnapshotterVolume string
	// SocketPath is the path to the containerd socket.
	SocketPath string
}

// AddFlagsToCommand will add the containerd image service specific flags to the supplied cobra command.
func AddFlagsToCommand(cmd *cobra.Command, config *Config) error {
	cmd.Flags().StringVar(&config.SocketPath,
		socketPathFlagName,
		defaults.ContainerdSocket,
		"The path to the containerd socket.")

	cmd.Flags().StringVar(&config.SnapshotterKernel,
		kernelSnapshotterFlagName,
		defaults.ContainerdSnapshotter,
		fmt.Sprintf("The name of the snapshotter to use with containerd for kernel images. Options: %s", supportedSnapshotters))

	cmd.Flags().StringVar(&config.SnapshotterVolume,
		volSnapshotterFlagName,
		defaults.ContainerdSnapshotter,
		fmt.Sprintf("The name of the snapshotter to use with containerd for volume/initrd images. Options: %s", supportedSnapshotters))

	return nil
}
