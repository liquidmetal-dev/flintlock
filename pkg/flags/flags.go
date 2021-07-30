package flags

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// BindCommandFlagsToViper will bind the flags on a command to viper.
func BindCommandToViper(cmd *cobra.Command) {
	bindFlagsToViper(cmd.PersistentFlags())
	bindFlagsToViper(cmd.Flags())
}

func bindFlagsToViper(fs *pflag.FlagSet) {
	fs.VisitAll(func(flag *pflag.Flag) {
		viper.BindPFlag(flag.Name, flag) //nolint: errcheck
		viper.BindEnv(flag.Name)         //nolint: errcheck

		if !flag.Changed && viper.IsSet(flag.Name) {
			val := viper.Get(flag.Name)
			fs.Set(flag.Name, fmt.Sprintf("%v", val)) //nolint: errcheck
		}
	})
}
