package command

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v2"
	"github.com/urfave/cli/v2/altsrc"

	"github.com/weaveworks/flintlock/internal/command/gw"
	"github.com/weaveworks/flintlock/internal/command/run"
	"github.com/weaveworks/flintlock/internal/config"
	"github.com/weaveworks/flintlock/internal/version"
	"github.com/weaveworks/flintlock/pkg/defaults"
	"github.com/weaveworks/flintlock/pkg/log"
)

const usage = `
  __  _  _         _    _               _        _
 / _|| |(_) _ __  | |_ | |  ___    ___ | | __ __| |
| |_ | || || '_ \ | __|| | / _ \  / __|| |/ // _' |
|  _|| || || | | || |_ | || (_) || (__ |   <| (_| |
|_|  |_||_||_| |_| \__||_| \___/  \___||_|\_\\__,_|

Create and manage the lifecycle of MicroVMs, backed by containerd
`

func NewApp(out io.Writer) *cli.App {
	cfg := &config.Config{}
	// Append to the default template
	cli.AppHelpTemplate = fmt.Sprintf(`%s

		WEBSITE: https://docs.flintlock.dev/
		
		SUPPORT: https://github.com/weaveworks/flintlock
		
		`, cli.AppHelpTemplate)

	app := cli.NewApp()

	if out != nil {
		app.Writer = out
	}

	app.Name = "flintlockd"
	app.Usage = usage
	app.Description = `
flintlock is a service for creating and managing the lifecycle of microVMs on a host machine. 
Initially we will be supporting Firecracker.

The primary use case for flintlock is to create microVMs on a bare-metal host where the microVMs 
will be used as nodes in a virtualized Kubernetes cluster. It is an essential part of 
Liquid Metal and will ultimately be driven by Cluster API Provider Microvm (coming soon).

A default configuration is used if located at the default file location /etc/opt/flintlockd/config.yaml. It can also be set with XDG_CONFIG_HOME environment variable.
The file path has to be XDG_CONFIG_HOME/flintlockd/config.yaml.`
	app.HideVersion = true

	log.AddFlagsToApp(app, &cfg.Logging)
	addCommands(app, cfg)

	app.Action = func(c *cli.Context) error {
		err := cli.ShowAppHelp(c)
		if err != nil {
			return err
		}

		return nil
	}

	app.Before = func(context *cli.Context) error {
		// Load the configuration file
		var configPath string

		xdgCfg := os.Getenv("XDG_CONFIG_HOME")
		if xdgCfg != "" {
			configPath = filepath.Join(xdgCfg, "flintlockd", defaults.ConfigFile)
		} else {
			configPath = filepath.Join(defaults.ConfigurationDir, defaults.ConfigFile)
		}

		if _, err := os.Stat(configPath); err == nil {
			inputSource, err := altsrc.NewYamlSourceFromFile(configPath)
			if err != nil {
				return fmt.Errorf("unable to create input source with context: %w", err)
			}

			err = altsrc.ApplyInputSourceValues(context, inputSource, app.Flags)
			if err != nil {
				return fmt.Errorf("unable to apply input source with context: %w", err)
			}
		}

		if err := log.Configure(&cfg.Logging); err != nil {
			return fmt.Errorf("configuring logging: %w", err)
		}

		return nil
	}

	return app
}

func addCommands(app *cli.App, cfg *config.Config) {
	runCmd := run.NewCommand(cfg)
	gwCmd := gw.NewCommand(cfg)
	versionCmd := versionCommand()
	app.Commands = append(app.Commands, runCmd, gwCmd, versionCmd)
}

func versionCommand() *cli.Command {
	cmd := &cli.Command{
		Name:  "version",
		Usage: "Print the version number of flintlock",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "long",
				Usage: "Print the long version information",
				Value: false,
			},
			&cli.BoolFlag{
				Name:  "short",
				Usage: "Print the short version information",
				Value: false,
			},
		},
		Action: func(context *cli.Context) error {
			long := context.Bool("long")
			short := context.Bool("short")

			if short {
				fmt.Fprintln(context.App.Writer, version.Version)

				return nil
			}

			if long {
				fmt.Fprintf(
					context.App.Writer,
					"%s\n  Version:    %s\n  CommitHash: %s\n  BuildDate:  %s\n",
					version.PackageName,
					version.Version,
					version.CommitHash,
					version.BuildDate,
				)

				return nil
			}

			fmt.Fprintf(context.App.Writer, "%s %s\n", version.PackageName, version.Version)

			return nil
		},
	}

	return cmd
}
