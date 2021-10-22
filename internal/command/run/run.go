package run

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	mvmv1 "github.com/weaveworks/flintlock/api/services/microvm/v1alpha1"
	cmdflags "github.com/weaveworks/flintlock/internal/command/flags"
	"github.com/weaveworks/flintlock/internal/config"
	"github.com/weaveworks/flintlock/internal/inject"
	"github.com/weaveworks/flintlock/pkg/defaults"
	"github.com/weaveworks/flintlock/pkg/flags"
	"github.com/weaveworks/flintlock/pkg/log"
)

// NewCommand creates a new cobra command for running flintlock.
func NewCommand(cfg *config.Config) (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Start running the flintlock API",
		PreRunE: func(c *cobra.Command, _ []string) error {
			flags.BindCommandToViper(c)

			return nil
		},
		RunE: func(c *cobra.Command, _ []string) error {
			return runServer(c.Context(), cfg)
		},
	}

	cmdflags.AddGRPCServerFlagsToCommand(cmd, cfg)
	if err := cmdflags.AddContainerDFlagsToCommand(cmd, cfg); err != nil {
		return nil, fmt.Errorf("adding containerd flags to run command: %w", err)
	}
	if err := cmdflags.AddFirecrackerFlagsToCommand(cmd, cfg); err != nil {
		return nil, fmt.Errorf("adding firecracker flags to run command: %w", err)
	}
	if err := cmdflags.AddNetworkFlagsToCommand(cmd, cfg); err != nil {
		return nil, fmt.Errorf("adding network flags to run command: %w", err)
	}
	if err := cmdflags.AddHiddenFlagsToCommand(cmd, cfg); err != nil {
		return nil, fmt.Errorf("adding hidden flags to run command: %w", err)
	}
	cmd.Flags().StringVar(&cfg.StateRootDir, "state-dir", defaults.StateRootDir, "The directory to use for the as the root for runtime state.")
	cmd.Flags().DurationVar(&cfg.ResyncPeriod, "resync-period", defaults.ResyncPeriod, "Reconcile the specs to resynchronise them based on this period.")

	return cmd, nil
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
		return fmt.Errorf("initializing ports for application: %w", err)
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
	l, err := net.Listen("tcp", cfg.GRPCAPIEndpoint)
	if err != nil {
		return fmt.Errorf("setting up gRPC api listener: %w", err)
	}
	defer l.Close()

	reflection.Register(grpcServer)

	if err := grpcServer.Serve(l); err != nil {
		logger.Fatalf("serving grpc api: %v", err) // TODO: remove this fatal
	}

	return nil
}

func runControllers(ctx context.Context, cfg *config.Config) error {
	logger := log.GetLogger(ctx)

	ports, err := inject.InitializePorts(cfg)
	if err != nil {
		return fmt.Errorf("initializing ports for controller: %w", err)
	}
	app := inject.InitializeApp(cfg, ports)
	mvmControllers := inject.InializeController(app, ports)

	logger.Info("starting microvm controller")
	if err := mvmControllers.Run(ctx, 1, cfg.ResyncPeriod, true); err != nil {
		logger.Fatalf("starting microvm controller: %v", err)
	}

	return nil
}
