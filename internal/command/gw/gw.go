package gw

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"

	mvmv1 "github.com/weaveworks/flintlock/api/services/microvm/v1alpha1"
	cmdflags "github.com/weaveworks/flintlock/internal/command/flags"
	"github.com/weaveworks/flintlock/internal/config"
	"github.com/weaveworks/flintlock/internal/version"
	"github.com/weaveworks/flintlock/pkg/flags"
	"github.com/weaveworks/flintlock/pkg/log"
)

// NewCommand creates a new cobra command for running the gRPC HTTP gateway.
func NewCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gw",
		Short: "Start serving the HTTP gateway for the flintlock gRPC API",
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
			return runGWServer(c.Context(), cfg)
		},
	}

	cmdflags.AddGWServerFlagsToCommand(cmd, cfg)

	return cmd
}

func runGWServer(ctx context.Context, cfg *config.Config) error {
	logger := log.GetLogger(ctx)
	logger.Info("flintlockd grpc api gateway starting")

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

	<-sigChan
	logger.Debug("shutdown signal received, waiting for work to finish")

	cancel()
	wg.Wait()

	logger.Info("all work finished, exiting")

	return nil
}

func serveAPI(ctx context.Context, cfg *config.Config) error {
	logger := log.GetLogger(ctx)
	mux := runtime.NewServeMux()

	opts := []grpc.DialOption{
		grpc.WithInsecure(),
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
