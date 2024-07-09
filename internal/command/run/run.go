package run

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	_ "net/http/pprof"

	grpc_mw "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_auth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	mvmv1 "github.com/liquidmetal-dev/flintlock/api/services/microvm/v1alpha1"
	"github.com/liquidmetal-dev/flintlock/infrastructure/microvm"
	cmdflags "github.com/liquidmetal-dev/flintlock/internal/command/flags"
	"github.com/liquidmetal-dev/flintlock/internal/config"
	"github.com/liquidmetal-dev/flintlock/internal/inject"
	"github.com/liquidmetal-dev/flintlock/internal/version"
	"github.com/liquidmetal-dev/flintlock/pkg/auth"
	"github.com/liquidmetal-dev/flintlock/pkg/flags"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

// NewCommand creates a new cobra command for running flintlock.
func NewCommand(cfg *config.Config) (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Start running the flintlock API",
		PreRunE: func(c *cobra.Command, _ []string) error {
			flags.BindCommandToViper(c)

			logger := log.GetLogger(c.Context())
			logger.Infof(
				"flintlockd, version=%s, built_on=%s, commit=%s",
				version.Version,
				version.BuildDate,
				version.CommitHash,
			)

			if cfg.ParentIface == "" && cfg.BridgeName == "" {
				return errors.New("You must supply at least one of parent interface, bridge name")
			}

			providerFound := false
			for _, supportedProvider := range microvm.GetProviderNames() {
				if supportedProvider == cfg.DefaultVMProvider {
					providerFound = true
					break
				}
			}
			if !providerFound {
				return fmt.Errorf("The provided default provider name %s isn't a supported provider", cfg.DefaultVMProvider)
			}
			logger.Infof("Default microvm provider: %s", cfg.DefaultVMProvider)

			return nil
		},
		RunE: func(c *cobra.Command, _ []string) error {
			return runServer(c.Context(), cfg)
		},
	}

	cmdflags.AddGRPCServerFlagsToCommand(cmd, cfg)
	cmdflags.AddAuthFlagsToCommand(cmd, cfg)
	cmdflags.AddTLSFlagsToCommand(cmd, cfg)
	cmdflags.AddContainerDFlagsToCommand(cmd, cfg)
	cmdflags.AddMicrovmProviderFlagsToCommand(cmd, cfg)
	cmdflags.AddDebugFlagsToCommand(cmd, cfg)
	cmdflags.AddGWServerFlagsToCommand(cmd, cfg)

	if err := cmdflags.AddNetworkFlagsToCommand(cmd, cfg); err != nil {
		return nil, fmt.Errorf("adding network flags to run command: %w", err)
	}

	if err := cmdflags.AddHiddenFlagsToCommand(cmd, cfg); err != nil {
		return nil, fmt.Errorf("adding hidden flags to run command: %w", err)
	}

	return cmd, nil
}

func runServer(ctx context.Context, cfg *config.Config) error {
	logger := log.GetLogger(ctx)
	logger.Info("flintlockd grpc api server starting")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	wg := &sync.WaitGroup{}
	ctx, cancel := context.WithCancel(log.WithLogger(ctx, logger))

	if cfg.DebugEndpoint != "" {
		wg.Add(1)

		go func() {
			defer wg.Done()

			if err := runPProf(ctx, cfg); err != nil {
				logger.Errorf("failed serving api: %v", err)
				// Cancel all processes if at least one fails.
				cancel()
			}
		}()
	}

	if !cfg.DisableAPI {
		wg.Add(1)

		go func() {
			defer wg.Done()

			if err := serveAPI(ctx, cfg); err != nil {
				logger.Errorf("failed serving api: %v", err)
				// Cancel all processes if at least one fails.
				cancel()
			}
		}()
	}

	if cfg.EnableHTTPGateway {
		wg.Add(1)

		go func() {
			defer wg.Done()
			if err := serveHTTP(ctx, cfg); err != nil {
				logger.Errorf("failed serving http api: %v", err)
				// Cancel all processes if at least one fails.
				cancel()
			}
		}()
	}

	if !cfg.DisableReconcile {
		wg.Add(1)

		go func() {
			defer wg.Done()

			if err := runControllers(ctx, cfg); err != nil {
				logger.Errorf("failed running controllers: %v", err)
				// Cancel all processes if at least one fails.
				cancel()
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

	if err := cfg.TLS.Validate(); err != nil {
		return fmt.Errorf("validating tls config: %w", err)
	}

	ports, err := inject.InitializePorts(cfg)
	if err != nil {
		return fmt.Errorf("initialising ports for application: %w", err)
	}

	app := inject.InitializeApp(cfg, ports)
	server := inject.InitializeGRPCServer(app)

	serverOpts, err := generateOpts(ctx, cfg)
	if err != nil {
		return err
	}

	grpcServer := grpc.NewServer(serverOpts...)

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
		return fmt.Errorf("failed to start grpc server: %w", err)
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

func generateOpts(ctx context.Context, cfg *config.Config) ([]grpc.ServerOption, error) {
	logger := log.GetLogger(ctx)

	opts := []grpc.ServerOption{
		grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor),
		grpc.UnaryInterceptor(grpc_prometheus.UnaryServerInterceptor),
	}

	if cfg.BasicAuthToken != "" {
		logger.Info("basic authentication is enabled")

		opts = []grpc.ServerOption{
			grpc.StreamInterceptor(grpc_mw.ChainStreamServer(
				grpc_prometheus.StreamServerInterceptor,
				grpc_auth.StreamServerInterceptor(auth.BasicAuthFunc(cfg.BasicAuthToken)),
			)),
			grpc.UnaryInterceptor(grpc_mw.ChainUnaryServer(
				grpc_prometheus.UnaryServerInterceptor,
				grpc_auth.UnaryServerInterceptor(auth.BasicAuthFunc(cfg.BasicAuthToken)),
			)),
		}
	} else {
		logger.Warn("basic authentication is DISABLED")
	}

	if !cfg.TLS.Insecure {
		logger.Info("TLS is enabled")

		creds, err := auth.LoadTLSForGRPC(&cfg.TLS)
		if err != nil {
			return nil, err
		}

		opts = append(opts, grpc.Creds(creds))
	} else {
		logger.Warn("TLS is DISABLED")
	}

	return opts, nil
}

func runPProf(ctx context.Context, cfg *config.Config) error {
	logger := log.GetLogger(ctx)
	logger.Warnf("Debug endpoint is ENABLED at %s", cfg.DebugEndpoint)

	srv := &http.Server{
		Addr:    cfg.DebugEndpoint,
		Handler: http.DefaultServeMux,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("starting debug endpoint: %v", err)
		}
	}()

	<-ctx.Done()
	logger.Debug("Exiting")

	shutDownCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer func() {
		cancel()
	}()

	if err := srv.Shutdown(shutDownCtx); err != nil {
		logger.Warnf("Debug server shutdown failed:%+v", err)
	}

	return nil
}

func serveHTTP(ctx context.Context, cfg *config.Config) error {
	logger := log.GetLogger(ctx)
	mux := runtime.NewServeMux()

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	if err := mvmv1.RegisterMicroVMHandlerFromEndpoint(ctx, mux, cfg.GRPCAPIEndpoint, opts); err != nil {
		return fmt.Errorf("could not register microvm server: %w", err)
	}

	server := &http.Server{
		Addr:    cfg.HTTPAPIEndpoint,
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		logger.Infof("shutting down the http gateway server")

		//nolint: contextcheck // Intentional.
		if err := server.Shutdown(context.Background()); err != nil {
			logger.Errorf("failed to shutdown http gateway server: %v", err)
		}
	}()

	logger.Debugf("starting http server listening on endpoint %s", cfg.HTTPAPIEndpoint)

	if err := server.ListenAndServe(); err != nil {
		return fmt.Errorf("listening and serving http api: %w", err)
	}

	return nil
}
