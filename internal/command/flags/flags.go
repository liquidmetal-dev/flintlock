package flags

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/weaveworks/flintlock/internal/config"
	"github.com/weaveworks/flintlock/pkg/defaults"
)

const (
	grpcEndpointFlag      = "grpc-endpoint"
	httpEndpointFlag      = "http-endpoint"
	parentIfaceFlag       = "parent-iface"
	disableReconcileFlag  = "disable-reconcile"
	disableAPIFlag        = "disable-api"
	firecrackerBinFlag    = "firecracker-bin"
	firecrackerDetachFlag = "firecracker-detach"
	firecrackerAPIFlag    = "firecracker-api"
	containerdSocketFlag  = "containerd-socket"
	volSnapshotterFlag    = "containerd-volume-ss"
	kernelSnapshotterFlag = "containerd-kernel-ss"
	containerdNamespace   = "containerd-ns"
	maximumRetryFlag      = "maximum-retry"
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

func AddNetworkFlagsToCommand(cmd *cobra.Command, cfg *config.Config) error {
	cmd.Flags().StringVar(&cfg.ParentIface,
		parentIfaceFlag,
		"",
		"The parent iface for the network interfaces. Note it could also be a bond")

	if err := cmd.MarkFlagRequired(parentIfaceFlag); err != nil {
		return fmt.Errorf("setting %s as required: %w", parentIfaceFlag, err)
	}

	return nil
}

func AddHiddenFlagsToCommand(cmd *cobra.Command, cfg *config.Config) error {
	cmd.Flags().BoolVar(&cfg.DisableReconcile,
		disableReconcileFlag,
		false,
		"Set to true to stop the reconciler running")

	cmd.Flags().IntVar(&cfg.MaximumRetry,
		maximumRetryFlag,
		defaults.MaximumRetry,
		"Number of times to retry failed reconciliation")

	cmd.Flags().BoolVar(&cfg.DisableAPI,
		disableAPIFlag,
		false,
		"Set to true to stop the api server running")

	if err := cmd.Flags().MarkHidden(disableReconcileFlag); err != nil {
		return fmt.Errorf("setting %s as hidden: %w", disableReconcileFlag, err)
	}
	if err := cmd.Flags().MarkHidden(maximumRetryFlag); err != nil {
		return fmt.Errorf("setting %s as hidden: %w", maximumRetryFlag, err)
	}
	if err := cmd.Flags().MarkHidden(disableAPIFlag); err != nil {
		return fmt.Errorf("setting %s as hidden: %w", disableAPIFlag, err)
	}

	return nil
}

// AddFirecrackerFlagsToCommand will add the firecracker provider specific flags to the supplied cobra command.
func AddFirecrackerFlagsToCommand(cmd *cobra.Command, cfg *config.Config) error {
	cmd.Flags().StringVar(&cfg.FirecrackerBin,
		firecrackerBinFlag,
		defaults.FirecrackerBin,
		"The path to the firecracker binary to use.")
	cmd.Flags().BoolVar(&cfg.FirecrackerDetatch,
		firecrackerDetachFlag,
		defaults.FirecrackerDetach,
		"If true the child firecracker processes will be detached from the parent flintlock process.")
	cmd.Flags().BoolVar(&cfg.FirecrackerUseAPI,
		firecrackerAPIFlag,
		defaults.FirecrackerUseAPI,
		"Indicates that the Firecracker API should be used to configure the microvm.")

	return nil
}

// AddContainerDFlagsToCommand will add the containerd specific flags to the supplied cobra command.
func AddContainerDFlagsToCommand(cmd *cobra.Command, cfg *config.Config) error {
	cmd.Flags().StringVar(&cfg.CtrSocketPath,
		containerdSocketFlag,
		defaults.ContainerdSocket,
		"The path to the containerd socket.")

	cmd.Flags().StringVar(&cfg.CtrSnapshotterKernel,
		kernelSnapshotterFlag,
		defaults.ContainerdKernelSnapshotter,
		"The name of the snapshotter to use with containerd for kernel/initrd images.")

	cmd.Flags().StringVar(&cfg.CtrSnapshotterVolume,
		volSnapshotterFlag,
		defaults.ContainerdVolumeSnapshotter,
		"The name of the snapshotter to use with containerd for volume images.")

	cmd.Flags().StringVar(&cfg.CtrNamespace,
		containerdNamespace,
		defaults.ContainerdNamespace,
		"The name of the containerd namespace to use.")

	return nil
}
