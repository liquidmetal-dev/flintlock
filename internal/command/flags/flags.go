package flags

import (
	"github.com/spf13/cobra"

	"github.com/weaveworks/reignite/internal/config"
	"github.com/weaveworks/reignite/pkg/defaults"
)

const (
	grpcEndpointFlag = "grpc-endpoint"
	httpEndpointFlag = "http-endpoint"
)

// AddGRPCServerFlagsToCommand will add gRPC server flags to the supplied command.
func AddGRPCServerFlagsToCommand(cmd *cobra.Command, cfg *config.Config) {
	cmd.Flags().StringVar(&cfg.GRPCAPIEndpoint,
		grpcEndpointFlag,
		defaults.GRPCAPIEndpoint,
		"The endpoint for the gRPC server to listen on.")
}

// AddGWServerFlagsToCommand will add gRPC HTTP gateway flags to the supplied command.
func AddGWServerFlagsToCommand(cmd *cobra.Command, cfg *config.Config) {
	cmd.Flags().StringVar(&cfg.GRPCAPIEndpoint,
		grpcEndpointFlag,
		defaults.GRPCAPIEndpoint,
		"The address of the gRPC server to act as a gateway for.")

	cmd.Flags().StringVar(&cfg.HTTPAPIEndpoint,
		httpEndpointFlag,
		defaults.HTTPAPIEndpoint,
		"The endpoint for the HTTP proxy to the gRPC service to listen on.")
}
