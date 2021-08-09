package command

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/weaveworks/reignite/pkg/defaults"
	"github.com/weaveworks/reignite/pkg/flags"
	"github.com/weaveworks/reignite/pkg/log"
)

func NewRootCommand() (*cobra.Command, error) {
	cfg := &Config{}

	cmd := &cobra.Command{
		Use:   "reignited",
		Short: "The reignite API",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			flags.BindCommandToViper(cmd)
			if err := log.Configure(&cfg.Logging); err != nil {
				return fmt.Errorf("configuring logging: %w", err)
			}

			return nil
		},
		RunE: func(c *cobra.Command, _ []string) error {
			return c.Help() //nolint: wrapcheck
		},
	}

	log.AddFlagsToCommand(cmd, &cfg.Logging)
	addRootSubCommands(cmd, cfg)

	cobra.OnInitialize(initCobra)

	return cmd, nil
}

func initCobra() {
	viper.SetEnvPrefix("REIGNITED")
	viper.AutomaticEnv()
	viper.SetConfigType("yaml")
	viper.SetConfigName("config")
	viper.AddConfigPath(defaults.ConfigurationDir)
	xdgCfg := os.Getenv("XDG_CONFIG_HOME")
	if xdgCfg != "" {
		viper.AddConfigPath("$XDG_CONFIG_HOME/reignited/")
	} else {
		viper.AddConfigPath("$HOME/.config/reignited/")
	}
	viper.ReadInConfig() //nolint: errcheck
}

func addRootSubCommands(cmd *cobra.Command, cfg *Config) {
	runCmd := newRunCommand(cfg)
	cmd.AddCommand(runCmd)
}
