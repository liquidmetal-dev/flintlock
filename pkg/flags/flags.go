package flags

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// BindCommandToViper will bind the flags on a command to viper.
func BindCommandToViper(cmd *cobra.Command) {
	bindFlagsToViper(cmd.PersistentFlags())
	bindFlagsToViper(cmd.Flags())
}

func bindFlagsToViper(fs *pflag.FlagSet) {
	fs.VisitAll(func(flag *pflag.Flag) {
		_ = viper.BindPFlag(flag.Name, flag)
		_ = viper.BindEnv(flag.Name)

		if !flag.Changed && viper.IsSet(flag.Name) {
			val := viper.Get(flag.Name)
			_ = fs.Set(flag.Name, fmt.Sprintf("%v", val))
		}
	})
}
