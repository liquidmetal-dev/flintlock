package flags

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/liquidmetal-dev/flintlock/infrastructure/microvm/firecracker"
	"github.com/liquidmetal-dev/flintlock/internal/config"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
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
	basicAuthTokenFlag        = "basic-auth-token" //nolint: gosec // This is a flag name
	insecureFlag              = "insecure"
	tlsCertFlag               = "tls-cert"
	tlsKeyFlag                = "tls-key"
	tlsClientValidateFlag     = "tls-client-validate"
	tlsClientCAFlag           = "tls-client-ca"
	debugEndpointFlag         = "debug-endpoint"
	cloudHypervisorBinFlag    = "cloudhypervisor-bin"
	cloudHypervisorDetachFlag = "cloudhypervisor-detach"
	virtioFSBinFlag           = "virtiofs-bin"
	repositoryStoreFlag       = "repository-store"
	sqliteDataPathFlag        = "sqlite-data-path"
	stateDirFlag              = "state-dir"
)

// AddGRPCServerFlagsToCommand will add gRPC server flags to the supplied command.
func AddGRPCServerFlagsToCommand(cmd *cobra.Command, cfg *config.Config) {
	cmd.Flags().StringVar(&cfg.GRPCAPIEndpoint,
		grpcEndpointFlag,
		defaults.GRPCAPIEndpoint,
		"The endpoint for the gRPC server to listen on.")

	AddStateDirFlagToCommand(cmd, cfg)

	cmd.Flags().DurationVar(&cfg.ResyncPeriod,
		"resync-period",
		defaults.ResyncPeriod,
		"Reconcile the specs to resynchronise them based on this period.")

	cmd.Flags().DurationVar(&cfg.DeleteVMTimeout,
		"deleteMicroVM-timeout",
		defaults.DeleteVMTimeout,
		"The timeout for deleting a microvm.")
}

// AddGWServerFlagsToCommand will add gRPC HTTP gateway flags to the supplied command.
func AddGWServerFlagsToCommand(cmd *cobra.Command, cfg *config.Config) {
	cmd.Flags().BoolVar(&cfg.EnableHTTPGateway,
		"enable-http",
		false,
		"Should the API be exposed via HTTP.")

	cmd.Flags().StringVar(&cfg.HTTPAPIEndpoint,
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

// AddNetworkFlagsToCommand will add various network flags to the command.
func AddNetworkFlagsToCommand(cmd *cobra.Command, cfg *config.Config) error {
	cmd.Flags().StringVar(&cfg.ParentIface,
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

// AddHiddenFlagsToCommand will add hidden flags to the supplied command.
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

// AddMicrovmProviderFlagsToCommand will add the microvm provider flags to the supplied command.
func AddMicrovmProviderFlagsToCommand(cmd *cobra.Command, cfg *config.Config) {
	addFirecrackerFlagsToCommand(cmd, cfg)
	addCloudHypervisorFlagsToCommand(cmd, cfg)

	cmd.Flags().StringVar(&cfg.DefaultVMProvider, "default-provider", firecracker.ProviderName, "The name of the microvm provider to use by default if not supplied in the create request.")
}

// AddContainerDFlagsToCommand will add the containerd specific flags to the supplied cobra command.
func AddContainerDFlagsToCommand(cmd *cobra.Command, cfg *config.Config) {
	cmd.Flags().StringVar(&cfg.CtrSocketPath,
		containerdSocketFlag,
		defaults.ContainerdSocket,
		"The path to the containerd socket.")

	cmd.Flags().StringVar(&cfg.CtrSnapshotterKernel,
		kernelSnapshotterFlag,
		defaults.ContainerdKernelSnapshotter,
		"The name of the snapshotter to use with containerd for kernel/initrd images.")

	cmd.Flags().StringVar(&cfg.CtrNamespace,
		containerdNamespace,
		defaults.ContainerdNamespace,
		"The name of the containerd namespace to use.")
}

// AddStateDirFlagToCommand will add the --state-dir flag to the supplied command.
func AddStateDirFlagToCommand(cmd *cobra.Command, cfg *config.Config) {
	cmd.Flags().StringVar(&cfg.StateRootDir,
		stateDirFlag,
		defaults.StateRootDir,
		"The directory to use for the as the root for runtime state.")
}

// AddRepositoryFlagsToCommand will add the microvm repository backing-store flags to the supplied command.
func AddRepositoryFlagsToCommand(cmd *cobra.Command, cfg *config.Config) {
	cmd.Flags().StringVar(&cfg.RepositoryStore,
		repositoryStoreFlag,
		defaults.RepositoryStore,
		fmt.Sprintf("The backing store to use for microvm spec/status definitions (%q or %q).", "containerd", "sqlite"))

	cmd.Flags().StringVar(&cfg.SqliteDataPath,
		sqliteDataPathFlag,
		"",
		"The path to the sqlite database file to use when --"+repositoryStoreFlag+"=sqlite. "+
			"Defaults to a file under --"+stateDirFlag+".")
}

func AddDebugFlagsToCommand(cmd *cobra.Command, cfg *config.Config) {
	cmd.Flags().StringVar(&cfg.DebugEndpoint,
		debugEndpointFlag,
		"",
		"The endpoint for the debug web server to listen on. It must include a port (e.g. localhost:10500).  An empty string means disable the debug endpoint.")
}

func AddVirtioFSFlagsToCommand(cmd *cobra.Command, cfg *config.Config) {
	cmd.Flags().StringVar(&cfg.VirtioFSBin,
		virtioFSBinFlag,
		defaults.VirtioFSBin,
		"The path to the virtiofs binary to use.")
}

func addFirecrackerFlagsToCommand(cmd *cobra.Command, cfg *config.Config) {
	cmd.Flags().StringVar(&cfg.FirecrackerBin,
		firecrackerBinFlag,
		defaults.FirecrackerBin,
		"The path to the firecracker binary to use.")
	cmd.Flags().BoolVar(&cfg.FirecrackerDetatch,
		firecrackerDetachFlag,
		defaults.FirecrackerDetach,
		"If true the child firecracker processes will be detached from the parent flintlock process.")
}

func addCloudHypervisorFlagsToCommand(cmd *cobra.Command, cfg *config.Config) {
	cmd.Flags().StringVar(&cfg.CloudHypervisorBin,
		cloudHypervisorBinFlag,
		defaults.CloudHypervisorBin,
		"The path to the cloud hypervisor binary to use.")
	cmd.Flags().BoolVar(&cfg.CloudHypervisorDetatch,
		cloudHypervisorDetachFlag,
		defaults.CloudHypervisorDetach,
		"If true the child cloud hypervisor processes will be detached from the parent flintlock process.")
}
