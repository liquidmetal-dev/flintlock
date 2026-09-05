// Package provision wires the flintlock-provision cobra commands to the
// business logic in internal/provision.
package provision

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/liquidmetal-dev/flintlock/internal/provision"
	"github.com/liquidmetal-dev/flintlock/internal/version"
)

// NewRootCommand builds the flintlock-provision root command.
func NewRootCommand() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "flintlock-provision",
		Short: "Provision a host for running flintlock microvms",
		Long: "flintlock-provision bootstraps a host to run flintlock: installing " +
			"Firecracker, Cloud Hypervisor, containerd and flintlockd, and setting up " +
			"the devicemapper thinpool containerd needs.",
		RunE: func(c *cobra.Command, _ []string) error {
			return c.Help()
		},
	}

	cmd.AddCommand(
		newAllCommand(),
		newAptCommand(),
		newFirecrackerCommand(),
		newCloudHypervisorCommand(),
		newContainerdCommand(),
		newFlintlockCommand(),
		newDirectLVMCommand(),
		newDevPoolCommand(),
		newVersionCommand(),
	)

	return cmd, nil
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version number of flintlock-provision",
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "%s-provision %s\n", version.PackageName, version.Version)

			return nil
		},
	}
}

// hostArch returns the normalised (amd64/arm64) architecture of the host
// this command is running on.
func hostArch() (string, error) {
	return provision.NormaliseArch(runtime.GOARCH)
}
