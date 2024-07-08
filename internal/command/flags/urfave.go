package flags

import (
	"github.com/liquidmetal-dev/flintlock/internal/config"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
	"github.com/urfave/cli/v2"
)

type WithFlagsFunc func() []cli.Flag

func CLIFlags(options ...WithFlagsFunc) []cli.Flag {
	flags := []cli.Flag{}

	for _, group := range options {
		flags = append(flags, group()...)
	}

	return flags
}

func WithContainerDFlags() WithFlagsFunc {
	return func() []cli.Flag {
		return []cli.Flag{
			&cli.StringFlag{
				Name:  containerdSocketFlag,
				Value: defaults.ContainerdSocket,
				Usage: "The path to the containerd socket.",
			},
			&cli.StringFlag{
				Name:  containerdNamespace,
				Value: defaults.ContainerdNamespace,
				Usage: "The name of the containerd namespace to use.",
			},
		}
	}
}

func WithHTTPEndpointFlags() WithFlagsFunc {
	return func() []cli.Flag {
		return []cli.Flag{
			&cli.StringFlag{
				Name:  httpEndpointFlag,
				Value: defaults.HTTPAPIEndpoint,
				Usage: "The endpoint for the HTTP server to listen on.",
			},
		}
	}
}

func WithGlobalConfigFlags() WithFlagsFunc {
	return func() []cli.Flag {
		return []cli.Flag{
			&cli.StringFlag{
				Name:  "state-dir",
				Value: defaults.StateRootDir,
				Usage: "The directory to use for the as the root for runtime state.",
			},
		}
	}
}

func ParseFlags(cfg *config.Config) cli.BeforeFunc {
	return func(ctx *cli.Context) error {
		cfg.HTTPAPIEndpoint = ctx.String(httpEndpointFlag)

		cfg.CtrSocketPath = ctx.String(containerdSocketFlag)
		cfg.CtrNamespace = ctx.String(containerdNamespace)

		cfg.StateRootDir = ctx.String("state-dir")

		return nil
	}
}
