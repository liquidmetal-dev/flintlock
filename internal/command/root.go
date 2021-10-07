package command

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/weaveworks/reignite/internal/command/gw"
	"github.com/weaveworks/reignite/internal/command/run"
	"github.com/weaveworks/reignite/internal/config"
	"github.com/weaveworks/reignite/internal/version"
	"github.com/weaveworks/reignite/pkg/defaults"
	"github.com/weaveworks/reignite/pkg/flags"
	"github.com/weaveworks/reignite/pkg/log"
)

func NewRootCommand() (*cobra.Command, error) {
	cfg := &config.Config{}

	cmd := &cobra.Command{
		Use:   "reignited",
		Short: "The reignite API",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			flags.BindCommandToViper(cmd)
			if err := log.Configure(&cfg.Logging); err != nil {
				return fmt.Errorf("configuring logging: %w", err)
			}
			logger := log.GetLogger(cmd.Context())
			logger.Infof("reignited, version=%s, built_on=%s, commit=%s", version.Version, version.BuildDate, version.CommitHash)

			return nil
		},
		RunE: func(c *cobra.Command, _ []string) error {
			return c.Help() //nolint: wrapcheck
		},
	}

	log.AddFlagsToCommand(cmd, &cfg.Logging)
	if err := addRootSubCommands(cmd, cfg); err != nil {
		return nil, fmt.Errorf("adding subcommands: %w", err)
	}

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

func addRootSubCommands(cmd *cobra.Command, cfg *config.Config) error {
	runCmd, err := run.NewCommand(cfg)
	if err != nil {
		return fmt.Errorf("creating run cobra command: %w", err)
	}
	cmd.AddCommand(runCmd)

	gwCmd := gw.NewCommand(cfg)
	cmd.AddCommand(gwCmd)

	return nil
}
