package flags

import (
	"github.com/urfave/cli/v2"
	"github.com/urfave/cli/v2/altsrc"

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
func AddGRPCServerFlagsToCommand(cmd *cli.Command, cfg *config.Config) {
	cmd.Flags = append(cmd.Flags, altsrc.NewStringFlag(&cli.StringFlag{
		Name:        grpcEndpointFlag,
		Usage:       "The endpoint for the gRPC server to listen on.",
		Value:       defaults.GRPCAPIEndpoint,
		Destination: &cfg.GRPCAPIEndpoint,
	}))
}

// AddGWServerFlagsToCommand will add gRPC HTTP gateway flags to the supplied command.
func AddGWServerFlagsToCommand(cmd *cli.Command, cfg *config.Config) {
	grpcFlag := altsrc.NewStringFlag(&cli.StringFlag{
		Name:        grpcEndpointFlag,
		Usage:       "The address of the gRPC server to act as a gateway for.",
		Value:       defaults.GRPCAPIEndpoint,
		Destination: &cfg.GRPCAPIEndpoint,
	})

	httpFlag := altsrc.NewStringFlag(&cli.StringFlag{
		Name:        httpEndpointFlag,
		Usage:       "The endpoint for the HTTP proxy to the gRPC service to listen on.",
		Value:       defaults.HTTPAPIEndpoint,
		Destination: &cfg.HTTPAPIEndpoint,
	})

	cmd.Flags = append(cmd.Flags, grpcFlag, httpFlag)
}

func AddNetworkFlagsToCommand(cmd *cli.Command, cfg *config.Config) {
	cmd.Flags = append(cmd.Flags, altsrc.NewStringFlag(&cli.StringFlag{
		Name:        parentIfaceFlag,
		Usage:       "The parent iface for the network interfaces. Note it could also be a bond.",
		Value:       "",
		Destination: &cfg.ParentIface,
	}))
}

func AddHiddenFlagsToCommand(cmd *cli.Command, cfg *config.Config) {
	disableReconcileFlag := altsrc.NewBoolFlag(&cli.BoolFlag{
		Name:        disableReconcileFlag,
		Usage:       "Set to true to disable the reconciler.",
		Value:       false,
		Hidden:      true,
		Destination: &cfg.DisableReconcile,
	})

	maximumRetryFlag := altsrc.NewIntFlag(&cli.IntFlag{
		Name:        maximumRetryFlag,
		Usage:       "The maximum number of times to retry a failed reconciliation.",
		Value:       defaults.MaximumRetry,
		Hidden:      true,
		Destination: &cfg.MaximumRetry,
	})

	disableAPIFlag := altsrc.NewBoolFlag(&cli.BoolFlag{
		Name:        disableAPIFlag,
		Usage:       "Set to true to disable the API server.",
		Value:       false,
		Hidden:      true,
		Destination: &cfg.DisableAPI,
	})

	cmd.Flags = append(cmd.Flags, disableReconcileFlag, maximumRetryFlag, disableAPIFlag)
}

// AddFirecrackerFlagsToCommand will add the firecracker provider specific flags to the supplied cobra command.
func AddFirecrackerFlagsToCommand(cmd *cli.Command, cfg *config.Config) {
	firecrackerBinFlag := altsrc.NewStringFlag(&cli.StringFlag{
		Name:        firecrackerBinFlag,
		Usage:       "The path to the firecracker binary.",
		Value:       defaults.FirecrackerBin,
		Destination: &cfg.FirecrackerBin,
	})

	firecrackerDetachFlag := altsrc.NewBoolFlag(&cli.BoolFlag{
		Name:        firecrackerDetachFlag,
		Usage:       "Set to true to detach the firecracker process from the parent process.",
		Value:       defaults.FirecrackerDetach,
		Destination: &cfg.FirecrackerDetach,
	})

	firecrackerAPIFlag := altsrc.NewBoolFlag(&cli.BoolFlag{
		Name:        firecrackerAPIFlag,
		Usage:       "Set to true to enable the firecracker API to configure the microvm.",
		Value:       defaults.FirecrackerUseAPI,
		Destination: &cfg.FirecrackerUseAPI,
	})

	cmd.Flags = append(cmd.Flags, firecrackerBinFlag, firecrackerDetachFlag, firecrackerAPIFlag)
}

// AddContainerDFlagsToCommand will add the containerd specific flags to the supplied cobra command.
func AddContainerDFlagsToCommand(cmd *cli.Command, cfg *config.Config) {
	containerdSocketFlag := altsrc.NewPathFlag(&cli.PathFlag{
		Name:        containerdSocketFlag,
		Usage:       "The path to the containerd socket.",
		Value:       defaults.ContainerdSocket,
		Destination: &cfg.CtrSocketPath,
	})

	volSnapshotterFlag := altsrc.NewStringFlag(&cli.StringFlag{
		Name:        volSnapshotterFlag,
		Usage:       "The volume snapshotter to use.",
		Value:       defaults.ContainerdVolumeSnapshotter,
		Destination: &cfg.CtrVolumeSnapshotter,
	})

	kernelSnapshotterFlag := altsrc.NewStringFlag(&cli.StringFlag{
		Name:        kernelSnapshotterFlag,
		Usage:       "The kernel snapshotter to use.",
		Value:       defaults.ContainerdKernelSnapshotter,
		Destination: &cfg.CtrKernelSnapshotter,
	})

	containerdNamespaceFlag := altsrc.NewStringFlag(&cli.StringFlag{
		Name:        containerdNamespace,
		Usage:       "The containerd namespace to use.",
		Value:       defaults.ContainerdNamespace,
		Destination: &cfg.CtrNamespace,
	})

	cmd.Flags = append(cmd.Flags, containerdSocketFlag, volSnapshotterFlag, kernelSnapshotterFlag, containerdNamespaceFlag)
}
