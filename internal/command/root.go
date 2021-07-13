package command

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/weaveworks/reignite/pkg/defaults"
	"github.com/weaveworks/reignite/pkg/flags"
	"github.com/weaveworks/reignite/pkg/log"
	"github.com/weaveworks/reignite/pkg/microvm/firecracker"
)

func NewRootCommand() (*cobra.Command, error) {
	cfg := &Config{}
	cfg.MicroVM.Firecracker = &firecracker.Config{}

	cmd := &cobra.Command{
		Use:   "reignited",
		Short: "The reignite API",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			flags.BindCommandToViper(cmd)
			log.Configure(&cfg.Logging)

			logger := log.GetLogger(cmd.Context())
			logger.Info("reignite started")

			return nil
		},
		RunE: func(c *cobra.Command, _ []string) error {
			if err := c.Help(); err != nil {
				return err
			}

			return nil
		},
	}

	log.AddFlags(cmd.PersistentFlags(), &cfg.Logging)

	if err := addRootSubCommands(cmd, cfg); err != nil {
		return nil, fmt.Errorf("adding root subcommands: %w", err)
	}

	cobra.OnInitialize(initCobra)

	return cmd, nil
}

func initCobra() {
	viper.SetEnvPrefix("REIGNITED")
	viper.AutomaticEnv()
	viper.SetConfigType("yaml")
	viper.SetConfigName("config")
	viper.AddConfigPath(defaults.REIGNITED_CONF_DIR)
	xdgCfg := os.Getenv("XDG_CONFIG_HOME")
	if xdgCfg != "" {
		viper.AddConfigPath("$XDG_CONFIG_HOME/reignited/")
	} else {
		viper.AddConfigPath("$HOME/.config/reignited/")
	}
	viper.ReadInConfig()
}

func addRootSubCommands(cmd *cobra.Command, cfg *Config) error {
	runCmd, err := newRunCommand(cfg)
	if err != nil {
		return fmt.Errorf("creating run command: %w", err)
	}
	cmd.AddCommand(runCmd)

	return nil
}
