package run

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"

	"github.com/containerd/containerd"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"

	mvmv1 "github.com/weaveworks/reignite/api/services/microvm/v1alpha1"
	"github.com/weaveworks/reignite/core/application"
	reignite_ctr "github.com/weaveworks/reignite/infrastructure/containerd"
	"github.com/weaveworks/reignite/infrastructure/firecracker"
	microvmgrpc "github.com/weaveworks/reignite/infrastructure/grpc"
	"github.com/weaveworks/reignite/infrastructure/ulid"
	cmdflags "github.com/weaveworks/reignite/internal/command/flags"
	"github.com/weaveworks/reignite/internal/config"
	"github.com/weaveworks/reignite/pkg/flags"
	"github.com/weaveworks/reignite/pkg/log"
)

// NewCommand creates a new cobra command for running reignite.
func NewCommand(cfg *config.Config) (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Start running the reignite API",
		PreRunE: func(c *cobra.Command, _ []string) error {
			flags.BindCommandToViper(c)

			return nil
		},
		RunE: func(c *cobra.Command, _ []string) error {
			return runServer(c.Context(), cfg)
		},
	}

	cmdflags.AddGRPCServerFlagsToCommand(cmd, cfg)
	cmdflags.AddContainerdFlagsToCommand(cmd, cfg)
	if err := firecracker.AddFlagsToCommand(cmd, &cfg.Firecracker); err != nil {
		return nil, fmt.Errorf("adding firecracker flags to run command: %w", err)
	}

	return cmd, nil
}

func runServer(ctx context.Context, cfg *config.Config) error {
	logger := log.GetLogger(ctx)
	logger.Info("reignited grpc api server starting")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	wg := &sync.WaitGroup{}
	ctx, cancel := context.WithCancel(log.WithLogger(ctx, logger))

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := serveAPI(ctx, cfg); err != nil {
			logger.Errorf("failed serving api: %v", err)
		}
	}()

	// TODO: start the reconciler

	<-sigChan
	logger.Debug("shutdown signal received, waiting for work to finish")

	cancel()
	wg.Wait()

	logger.Info("all work finished, exiting")

	return nil
}

func serveAPI(ctx context.Context, cfg *config.Config) error {
	logger := log.GetLogger(ctx)

	// TODO: Use DI framework to inject these -------
	containerdClient, err := containerd.New(cfg.ContainerdSocketPath)
	if err != nil {
		return fmt.Errorf("creating containerd client: %w", err)
	}
	repo := reignite_ctr.NewMicroVMRepoWithClient(containerdClient)
	eventSvc := reignite_ctr.NewEventServiceWithClient(containerdClient)
	idSvc := ulid.New()
	mvmprovider := firecracker.New(&cfg.Firecracker)

	app := application.New(repo, eventSvc, idSvc, mvmprovider)
	server := microvmgrpc.NewServer(app, app)
	// END todo -----------------------------------------

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

	if err := grpcServer.Serve(l); err != nil {
		logger.Fatalf("serving grpc api: %v", err) // TODO: remove this fatal
	}

	return nil
}
