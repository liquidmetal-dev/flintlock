package state

import (
	"github.com/spf13/pflag"
	"github.com/weaveworks/reignite/pkg/defaults"
)

// AddFlags will add the state flags to the supplied flagset and bind to the provided config.
func AddFlags(fs *pflag.FlagSet, config *Config) {
	fs.StringVar(&config.Root, "state-root", defaults.STATE_DIR, "The directory to use for state storage.")
}
