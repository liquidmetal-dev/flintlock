package provision

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/liquidmetal-dev/flintlock/internal/provision"
)

func newFlintlockCommand() *cobra.Command {
	var (
		version     string
		address     string
		port        string
		parentIface string
		bridgeName  string
		insecure    bool
		configFile  string
		dev         bool
		binaryOnly  bool
	)

	cmd := &cobra.Command{
		Use:   "flintlock",
		Short: "Install and start flintlockd (note: will not succeed without containerd)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			arch, err := hostArch()
			if err != nil {
				return err
			}

			runner := provision.NewRunner(cmd.OutOrStdout(), cmd.ErrOrStderr())

			fmt.Fprintf(cmd.OutOrStdout(), "Installing flintlockd version %s to %s\n", version, provision.InstallPath)

			if binaryOnly {
				return provision.InstallFlintlockd(cmd.Context(), runner, version, arch)
			}

			containerdPaths := provision.BuildContainerdPaths(dev)

			return provision.AllFlintlock(cmd.Context(), runner, provision.AllFlintlockOptions{
				Version:              version,
				Address:              address,
				ParentIface:          parentIface,
				BridgeName:           bridgeName,
				Insecure:             insecure,
				ConfigFile:           configFile,
				Port:                 port,
				Arch:                 arch,
				ContainerdStateDir:   containerdPaths.StateDir,
				ContainerdSystemdSvc: containerdPaths.SystemdSvc,
			})
		},
	}

	cmd.Flags().StringVarP(&version, "version", "v", provision.VersionFromEnv(provision.FlintlockVersionEnv), "Version to install")
	cmd.Flags().StringVarP(&address, "grpc-address", "a", "",
		"Address on which to start the GRPC server (default: local ipv4 address)")
	cmd.Flags().StringVarP(&port, "grpc-port", "p", "", "Port on which to start the GRPC server (default: 9090)")
	cmd.Flags().StringVarP(&parentIface, "parent-iface", "i", "", "Interface of the default route of the host")
	cmd.Flags().StringVarP(&bridgeName, "bridge", "b", "",
		"Bridge to use instead of an interface (will override --parent-iface)")
	cmd.Flags().BoolVarP(&insecure, "insecure", "k", false, "Start flintlockd without basic auth or certs")
	cmd.Flags().StringVarP(&configFile, "config-file", "f", "",
		"Path to a valid flintlockd configuration file with overriding config")
	cmd.Flags().BoolVar(&dev, "dev", false, "Assumes containerd has been provisioned in a dev environment")
	cmd.Flags().BoolVar(&binaryOnly, "binary-only", false,
		"Only install the flintlockd binary, skipping the config file and systemd unit")

	return cmd
}
