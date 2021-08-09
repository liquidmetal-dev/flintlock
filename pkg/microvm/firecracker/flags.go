package firecracker

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/weaveworks/reignite/pkg/defaults"
)

const (
	socketFlagName         = "socket-path"
	firecrackerBinFlagName = "firecracker-bin"
)

// AddFlagsToCommand will add the firecracker provider specific flags to the supplied cobra command.
func AddFlagsToCommand(cmd *cobra.Command, config *Config) error {
	cmd.Flags().StringVar(&config.FirecrackerBin,
		firecrackerBinFlagName,
		defaults.FirecrackerBin,
		"The path to the firecracker binary to use.")
	cmd.Flags().StringVar(&config.SocketPath,
		socketFlagName,
		"",
		"The path to the directory to store the socket files in.")

	if err := cmd.MarkFlagRequired(socketFlagName); err != nil {
		return fmt.Errorf("setting %s as required: %w", socketFlagName, err)
	}

	return nil
}
