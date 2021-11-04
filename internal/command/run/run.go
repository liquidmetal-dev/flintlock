package run

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/urfave/cli/v2"
	"github.com/urfave/cli/v2/altsrc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	mvmv1 "github.com/weaveworks/flintlock/api/services/microvm/v1alpha1"
	cmdflags "github.com/weaveworks/flintlock/internal/command/flags"
	"github.com/weaveworks/flintlock/internal/config"
	"github.com/weaveworks/flintlock/internal/inject"
	"github.com/weaveworks/flintlock/internal/version"
	"github.com/weaveworks/flintlock/pkg/defaults"
	"github.com/weaveworks/flintlock/pkg/log"
)

var errParentIfaceRequired = errors.New("parent interface is required")

// NewCommand creates a new cobra command for running flintlock.
func NewCommand(cfg *config.Config) *cli.Command {
	cmd := &cli.Command{
		Name:  "run",
		Usage: "Run flintlock",
		Action: func(c *cli.Context) error {
			return runServer(c.Context, cfg)
		},
	}

	cmdflags.AddGRPCServerFlagsToCommand(cmd, cfg)
	cmdflags.AddContainerDFlagsToCommand(cmd, cfg)
	cmdflags.AddFirecrackerFlagsToCommand(cmd, cfg)
	cmdflags.AddNetworkFlagsToCommand(cmd, cfg)
	cmdflags.AddHiddenFlagsToCommand(cmd, cfg)

	stateRootDirFlag := altsrc.NewPathFlag(&cli.PathFlag{
		Name:        "state-dir",
		Usage:       "Path to the directory where flintlock will store its state",
		Value:       defaults.StateRootDir,
		Destination: &cfg.StateRootDir,
	})

	resyncPeriodflag := altsrc.NewDurationFlag(&cli.DurationFlag{
		Name:        "resync-period",
		Usage:       "Resync period for the specs reconciliation",
		Value:       defaults.ResyncPeriod,
		Destination: &cfg.ResyncPeriod,
	})

	cmd.Flags = append(cmd.Flags, stateRootDirFlag, resyncPeriodflag)

	cmd.Before = func(context *cli.Context) error {
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

			err = altsrc.ApplyInputSourceValues(context, inputSource, cmd.Flags)
			if err != nil {
				return fmt.Errorf("unable to apply input source with context: %w", err)
			}
		}

		if cfg.ParentIface == "" {
			err := cli.ShowCommandHelp(context, "run")
			if err != nil {
				return fmt.Errorf("unable to show command help: %w", err)
			}

			return errParentIfaceRequired
		}

		logger := log.GetLogger(context.Context)
		logger.Infof(
			"flintlockd, version=%s, built_on=%s, commit=%s",
			version.Version,
			version.BuildDate,
			version.CommitHash,
		)

		return nil
	}

	return cmd
}

func runServer(ctx context.Context, cfg *config.Config) error {
	logger := log.GetLogger(ctx)
	logger.Info("flintlockd grpc api server starting")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	wg := &sync.WaitGroup{}
	ctx, cancel := context.WithCancel(log.WithLogger(ctx, logger))

	if !cfg.DisableAPI {
		wg.Add(1)

		go func() {
			defer wg.Done()

			if err := serveAPI(ctx, cfg); err != nil {
				logger.Errorf("failed serving api: %v", err)
			}
		}()
	}

	if !cfg.DisableReconcile {
		wg.Add(1)

		go func() {
			defer wg.Done()

			if err := runControllers(ctx, cfg); err != nil {
				logger.Errorf("failed running controllers: %v", err)
			}
		}()
	}

	<-sigChan
	logger.Debug("shutdown signal received, waiting for work to finish")

	cancel()
	wg.Wait()

	logger.Info("all work finished, exiting")

	return nil
}

func serveAPI(ctx context.Context, cfg *config.Config) error {
	logger := log.GetLogger(ctx)

	ports, err := inject.InitializePorts(cfg)
	if err != nil {
		return fmt.Errorf("initialising ports for application: %w", err)
	}

	app := inject.InitializeApp(cfg, ports)
	server := inject.InitializeGRPCServer(app)

	grpcServer := grpc.NewServer(
		grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor),
		grpc.UnaryInterceptor(grpc_prometheus.UnaryServerInterceptor),
	)
	mvmv1.RegisterMicroVMServer(grpcServer, server)
	grpc_prometheus.Register(grpcServer)
	http.Handle("/metrics", promhttp.Handler())

	go func() {
		<-ctx.Done()
		logger.Infof("shutting down grpc server")
		grpcServer.GracefulStop()
	}()

	logger.Debugf("starting grpc server listening on endpoint %s", cfg.GRPCAPIEndpoint)

	listener, err := net.Listen("tcp", cfg.GRPCAPIEndpoint)
	if err != nil {
		return fmt.Errorf("setting up gRPC api listener: %w", err)
	}
	defer listener.Close()

	reflection.Register(grpcServer)

	if err := grpcServer.Serve(listener); err != nil {
		logger.Fatalf("serving grpc api: %v", err) // TODO: remove this fatal #235
	}

	return nil
}

func runControllers(ctx context.Context, cfg *config.Config) error {
	logger := log.GetLogger(ctx)

	ports, err := inject.InitializePorts(cfg)
	if err != nil {
		return fmt.Errorf("initialising ports for controller: %w", err)
	}

	app := inject.InitializeApp(cfg, ports)
	mvmControllers := inject.InializeController(app, ports)

	logger.Info("starting microvm controller")

	if err := mvmControllers.Run(ctx, 1, cfg.ResyncPeriod, true); err != nil {
		logger.Fatalf("starting microvm controller: %v", err)
	}

	return nil
}
