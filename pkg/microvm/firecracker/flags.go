package firecracker

import (
	"github.com/spf13/pflag"

	"github.com/weaveworks/reignite/pkg/defaults"
)

// AddFlags will add the firecracker provider specific flags to the flagset.
func AddFlags(fs *pflag.FlagSet, config *Config) {
	fs.StringVar(&config.FirecrackerBin, "firecracker-bin", defaults.FirecrackerBin, "The path to the firecracker binary to use.") //nolint:lll
	fs.StringVar(&config.SocketPath, "socket-path", "", "The path to the directory to store the socket files in.")
}
