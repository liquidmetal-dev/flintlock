package run

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"

	grpc_mw "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_auth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/spf13/cobra"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"

	mvmv1 "github.com/weaveworks/flintlock/api/services/microvm/v1alpha1"
	"github.com/weaveworks/flintlock/internal/auth"
	cmdflags "github.com/weaveworks/flintlock/internal/command/flags"
	"github.com/weaveworks/flintlock/internal/config"
	"github.com/weaveworks/flintlock/internal/inject"
	"github.com/weaveworks/flintlock/internal/version"
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

			logger := log.GetLogger(c.Context())
			logger.Infof(
				"flintlockd, version=%s, built_on=%s, commit=%s",
				version.Version,
				version.BuildDate,
				version.CommitHash,
			)

			return nil
		},
		RunE: func(c *cobra.Command, _ []string) error {
			return runServer(c.Context(), cfg)
		},
	}

	cmdflags.AddGRPCServerFlagsToCommand(cmd, cfg)
	cmdflags.AddTLSFlagsToCommand(cmd, cfg)

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
	cmd.Flags().DurationVar(&cfg.DeleteVMTimeout, "deleteMicroVM-timeout", defaults.DeleteVMTimeout, "The timeout for deleting a microvm.")

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

	if validErr := cfg.TLS.Validate(); validErr != nil {
		return fmt.Errorf("validating tls config: %w", validErr)
	}

	ports, err := inject.InitializePorts(cfg)
	if err != nil {
		return fmt.Errorf("initialising ports for application: %w", err)
	}

	app := inject.InitializeApp(cfg, ports)
	server := inject.InitializeGRPCServer(app)

	var opts []grpc.ServerOption
	if cfg.AuthToken != "" {
		opts = []grpc.ServerOption{
			grpc.StreamInterceptor(grpc_mw.ChainStreamServer(
				grpc_prometheus.StreamServerInterceptor,
				grpc_auth.StreamServerInterceptor(auth.BasicAuthFunc(cfg.AuthToken)),
			)),
			grpc.UnaryInterceptor(grpc_mw.ChainUnaryServer(
				grpc_prometheus.UnaryServerInterceptor,
				grpc_auth.UnaryServerInterceptor(auth.BasicAuthFunc(cfg.AuthToken)),
			)),
		}
	} else {
		opts = []grpc.ServerOption{
			grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor),
			grpc.UnaryInterceptor(grpc_prometheus.UnaryServerInterceptor),
		}
	}
	if !cfg.TLS.Insecure {
		creds, err := createTLSCreds(&cfg.TLS)

		if err != nil {
			return fmt.Errorf("creating tls credentials: %w", err)
		}
		opts = append(opts, grpc.Creds(creds))
	}

	grpcServer := grpc.NewServer(opts...)
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

func createTLSCreds(tlsCfg *config.TLSConfig) (credentials.TransportCredentials, error) {
	serverCert, err := tls.LoadX509KeyPair(tlsCfg.CertFile, tlsCfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("reading tls key pair: %w", err)
	}

	grpcTLS := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		MinVersion:   tls.VersionTLS13,
	}

	if tlsCfg.ClientCAFile != "" {
		caCert, err := ioutil.ReadFile(tlsCfg.ClientCAFile)
		if err != nil {
			return nil, fmt.Errorf("reading CA cert file %s: %w", tlsCfg.ClientCAFile, err)
		}

		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM(caCert)

		grpcTLS.ClientCAs = pool
	}

	if tlsCfg.ValidateClient {
		grpcTLS.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return credentials.NewTLS(grpcTLS), nil
}
