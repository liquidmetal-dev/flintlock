package firecracker

import (
	"github.com/spf13/pflag"

	"github.com/weaveworks/reignite/pkg/defaults"
)

func AddFlags(fs *pflag.FlagSet, config *Config) {
	fs.StringVar(&config.FirecrackerBin, "firecracker-bin", defaults.FIRECRACKER_BIN, "The path to the firecracker binary to use.")
	fs.StringVar(&config.SocketPath, "socket-path", "", "The path to the directory to store the socket files in.")
}
