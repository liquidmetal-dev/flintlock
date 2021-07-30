package command

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/spf13/cobra"

	mvmv1 "github.com/weaveworks/reignite/api/services/microvm/v1alpha1"
	"github.com/weaveworks/reignite/pkg/defaults"
	"github.com/weaveworks/reignite/pkg/flags"
	"github.com/weaveworks/reignite/pkg/log"
	"github.com/weaveworks/reignite/pkg/server"
)

func newRunCommand(cfg *Config) *cobra.Command {
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

	cmd.Flags().IntVarP(&cfg.PortNumber, "port", "p", defaults.APIPort, "The port number of the API server.")

	return cmd
}

func runServer(ctx context.Context, cfg *Config) error {
	logger := log.GetLogger(ctx)
	logger.Infof("reignited api server starting on port %d\n", cfg.PortNumber)

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

func serveAPI(ctx context.Context, cfg *Config) error {
	logger := log.GetLogger(ctx)
	mux := runtime.NewServeMux()

	// TODO: create the dependencies for the server

	if err := mvmv1.RegisterMicroVMHandlerServer(ctx, mux, server.NewServer()); err != nil {
		return fmt.Errorf("could not register microvm server: %w", err)
	}

	s := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.PortNumber),
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		logger.Infof("shutting down the http gateway server")
		if err := s.Shutdown(context.Background()); err != nil {
			logger.Errorf("failed to shutdown http gateway server: %v", err)
		}
	}()

	if err := s.ListenAndServe(); err != nil {
		return fmt.Errorf("listening and serving api: %w", err)
	}

	return nil
}
