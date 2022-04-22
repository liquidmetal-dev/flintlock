package flags

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/weaveworks-liquidmetal/flintlock/infrastructure/microvm/cloudhypervisor"
	"github.com/weaveworks-liquidmetal/flintlock/infrastructure/microvm/firecracker"
	"github.com/weaveworks-liquidmetal/flintlock/internal/config"
	"github.com/weaveworks-liquidmetal/flintlock/pkg/defaults"
)

const (
	grpcEndpointFlag          = "grpc-endpoint"
	httpEndpointFlag          = "http-endpoint"
	parentIfaceFlag           = "parent-iface"
	bridgeNameFlag            = "bridge-name"
	disableReconcileFlag      = "disable-reconcile"
	disableAPIFlag            = "disable-api"
	firecrackerBinFlag        = "firecracker-bin"
	firecrackerDetachFlag     = "firecracker-detach"
	containerdSocketFlag      = "containerd-socket"
	kernelSnapshotterFlag     = "containerd-kernel-ss"
	containerdNamespace       = "containerd-ns"
	maximumRetryFlag          = "maximum-retry"
	basicAuthTokenFlag        = "basic-auth-token"
	insecureFlag              = "insecure"
	tlsCertFlag               = "tls-cert"
	tlsKeyFlag                = "tls-key"
	tlsClientValidateFlag     = "tls-client-validate"
	tlsClientCAFlag           = "tls-client-ca"
	cloudHypervisorBinFlag    = "cloudhypervisor-bin"
	cloudHypervisorDetachFlag = "cloudhypervisor-detach"
)

var (
	errUnknownProvider = errors.New("unknown microvm provider name")
)

// AddGRPCServerPersistentFlags will add gRPC server flags to the supplied command.
func AddGRPCServerPersistentFlags(cmd *cobra.Command, cfg *config.Config) {
	cmd.PersistentFlags().StringVar(&cfg.GRPCAPIEndpoint,
		grpcEndpointFlag,
		defaults.GRPCAPIEndpoint,
		"The endpoint for the gRPC server to listen on.")

	cmd.Flags().StringVar(&cfg.StateRootDir,
		"state-dir",
		defaults.StateRootDir,
		"The directory to use for the as the root for runtime state.")

	cmd.Flags().DurationVar(&cfg.ResyncPeriod,
		"resync-period",
		defaults.ResyncPeriod,
		"Reconcile the specs to resynchronise them based on this period.")

	cmd.Flags().DurationVar(&cfg.DeleteVMTimeout,
		"deleteMicroVM-timeout",
		defaults.DeleteVMTimeout,
		"The timeout for deleting a microvm.")
}

// AddGWServerPersistentFlags will add gRPC HTTP gateway flags to the supplied command.
func AddGWServerPersistentFlags(cmd *cobra.Command, cfg *config.Config) {
	cmd.PersistentFlags().StringVar(&cfg.GRPCAPIEndpoint,
		grpcEndpointFlag,
		defaults.GRPCAPIEndpoint,
		"The address of the gRPC server to act as a gateway for.")

	cmd.PersistentFlags().StringVar(&cfg.HTTPAPIEndpoint,
		httpEndpointFlag,
		defaults.HTTPAPIEndpoint,
		"The endpoint for the HTTP proxy to the gRPC service to listen on.")
}

// AddAuthFlagsToCommand will add various auth method flags to the command.
func AddAuthFlagsToCommand(cmd *cobra.Command, cfg *config.Config) {
	cmd.Flags().StringVar(&cfg.BasicAuthToken,
		basicAuthTokenFlag,
		"",
		"The token to use for very basic token based authentication.")
}

// AddTLSFlagsToCommand will add TLS-related flags to the given command.
func AddTLSFlagsToCommand(cmd *cobra.Command, cfg *config.Config) {
	cmd.Flags().BoolVar(&cfg.TLS.Insecure,
		insecureFlag,
		false,
		"Run the gRPC server insecurely (i.e. without TLS). Not recommended.")

	cmd.Flags().StringVar(&cfg.TLS.CertFile,
		tlsCertFlag,
		"",
		"Path to the certificate to use for TLS.")

	cmd.Flags().StringVar(&cfg.TLS.KeyFile,
		tlsKeyFlag,
		"",
		"Path to the key to use for TLS.")

	cmd.Flags().BoolVar(&cfg.TLS.ValidateClient,
		tlsClientValidateFlag,
		false,
		"Validate the certificates of clients calling the gRPC server.")

	cmd.Flags().StringVar(&cfg.TLS.ClientCAFile,
		tlsClientCAFlag,
		"",
		"Path to the certificate to use when validating client certificates.")
}

// AddNetworkPersistentFlags will add various network flags to the command.
func AddNetworkPersistentFlags(cmd *cobra.Command, cfg *config.Config) error {
	cmd.PersistentFlags().StringVar(&cfg.ParentIface,
		parentIfaceFlag,
		"",
		"The parent iface for the network interfaces. Note it could also be a bond")

	cmd.Flags().StringVar(
		&cfg.BridgeName,
		bridgeNameFlag,
		"",
		"The name of the Linux bridge to attach tap devices to by default")

	return nil
}

func AddHiddenPersistentFlags(cmd *cobra.Command, cfg *config.Config) error {
	cmd.PersistentFlags().BoolVar(&cfg.DisableReconcile,
		disableReconcileFlag,
		false,
		"Set to true to stop the reconciler running")

	cmd.PersistentFlags().IntVar(&cfg.MaximumRetry,
		maximumRetryFlag,
		defaults.MaximumRetry,
		"Number of times to retry failed reconciliation")

	cmd.PersistentFlags().BoolVar(&cfg.DisableAPI,
		disableAPIFlag,
		false,
		"Set to true to stop the api server running")

	if err := cmd.PersistentFlags().MarkHidden(disableReconcileFlag); err != nil {
		return fmt.Errorf("setting %s as hidden: %w", disableReconcileFlag, err)
	}

	if err := cmd.PersistentFlags().MarkHidden(maximumRetryFlag); err != nil {
		return fmt.Errorf("setting %s as hidden: %w", maximumRetryFlag, err)
	}

	if err := cmd.PersistentFlags().MarkHidden(disableAPIFlag); err != nil {
		return fmt.Errorf("setting %s as hidden: %w", disableAPIFlag, err)
	}

	return nil
}

// AddContainerDPersistentFlags will add the containerd specific flags to the supplied cobra command.
func AddContainerDPersistentFlags(cmd *cobra.Command, cfg *config.Config) error {
	cmd.PersistentFlags().StringVar(&cfg.CtrSocketPath,
		containerdSocketFlag,
		defaults.ContainerdSocket,
		"The path to the containerd socket.")

	cmd.PersistentFlags().StringVar(&cfg.CtrSnapshotterKernel,
		kernelSnapshotterFlag,
		defaults.ContainerdKernelSnapshotter,
		"The name of the snapshotter to use with containerd for kernel/initrd images.")

	cmd.PersistentFlags().StringVar(&cfg.CtrNamespace,
		containerdNamespace,
		defaults.ContainerdNamespace,
		"The name of the containerd namespace to use.")

	return nil
}

func AddMicrovmServiceFlags(providerName string, cmd *cobra.Command, cfg *config.Config) error {
	switch providerName {
	case firecracker.ProviderName:
		return addFirecrackerFlags(cmd, cfg)
	case cloudhypervisor.ProviderName:
		return addCloudHypervisorFlags(cmd, cfg)
	default:
		return errUnknownProvider
	}
}

// addFirecrackerFlags will add the firecracker provider specific flags to the supplied cobra command.
func addFirecrackerFlags(cmd *cobra.Command, cfg *config.Config) error {
	cmd.Flags().StringVar(&cfg.FirecrackerBin,
		firecrackerBinFlag,
		defaults.FirecrackerBin,
		"The path to the firecracker binary to use.")
	cmd.Flags().BoolVar(&cfg.FirecrackerDetatch,
		firecrackerDetachFlag,
		defaults.FirecrackerDetach,
		"If true the child firecracker processes will be detached from the parent flintlock process.")

	return nil
}

// addCloudHypervisorFlags will add the Cloud Hypervisor provider specific flags to the supplied cobra command.
func addCloudHypervisorFlags(cmd *cobra.Command, cfg *config.Config) error {
	cmd.Flags().StringVar(&cfg.CloudHypervisorBin,
		cloudHypervisorBinFlag,
		defaults.CloudHypervisorBin,
		"The path to the cloud hypervisor binary to use.")
	cmd.Flags().BoolVar(&cfg.CloudHypervisorDetatch,
		cloudHypervisorDetachFlag,
		true,
		"If true the child cloud hypervisor processes will be detached from the parent flintlock process.")

	return nil
}
