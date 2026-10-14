package config

import (
	"time"

	"github.com/liquidmetal-dev/flintlock/internal/version"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
)

// Config represents the flintlockd configuration.
type Config struct {
	// ConfigFilePath is the path to the shared configuration file.
	ConfigFilePath string
	// Logging contains the logging related config.
	Logging log.Config
	// GRPCEndpoint is the endpoint for the gRPC server.
	GRPCAPIEndpoint string
	// HTTPAPIEndpoint is the endpoint for the HTTP proxy for the gRPC service
	HTTPAPIEndpoint string
	// EnableHTTPGateway indicates that the HTTP gateway should be started
	EnableHTTPGateway bool
	// FirecrackerBin is the firecracker binary to use.
	FirecrackerBin string
	// FirecrackerDetatch indicates if the child firecracker processes should be detached from their parent.
	FirecrackerDetatch bool
	// CloudHypervisorBin is the Cloud Hypervisor binary to use.
	CloudHypervisorBin string
	// VirtioFSBin is the VirtioFS binary to use.
	VirtioFSBin string
	// CloudHypervisorDetatch indicates if the child cloud hypervisor processes should be detached from their parent.
	CloudHypervisorDetatch bool
	// MkfsErofsPath is the path of the pinned mkfs.erofs used to build overlay base images.
	MkfsErofsPath string
	// OverlayRWSizeMB is the size in megabytes of the writable disk of an overlay volume
	// when the spec gives none.
	OverlayRWSizeMB int32
	// OverlayBaseRetention is how long an overlay base that no microvm uses is kept after
	// its last use.
	OverlayBaseRetention time.Duration
	// OverlayInitrdImage is the OCI image of the initrd that boots an overlay root. An empty
	// value selects the initrd published for the version of the daemon.
	OverlayInitrdImage string
	// StateRootDir is the directory to act as the root for the runtime state of flintlock.
	StateRootDir string
	// SocketDir is the directory to act as the root for the per-microvm unix sockets.
	SocketDir string
	// ParentIface is the name of the network interface to use for the parent in macvtap interfaces.
	ParentIface string
	// BridgeName is the name of the Linux bridge to attach tap devices to be default.
	BridgeName string
	// CtrSnapshotterKernel is the name of the containerd snapshotter to use for kernel images.
	CtrSnapshotterKernel string
	// CtrSocketPath is the path to the containerd socket.
	CtrSocketPath string
	// CtrNamespace is the default containerd namespace to use
	CtrNamespace string
	// CtrHostsDir is the certs.d directory of per-registry hosts.toml files used for image pulls.
	CtrHostsDir string
	// DisableReconcile is used to stop the reconcile part from running.
	DisableReconcile bool
	// DisableAPI is used to disable the api server.
	DisableAPI bool
	// ResyncPeriod defines the period when we should do a reconcile of the microvms (even if there are no events).
	ResyncPeriod time.Duration
	// MaximumRetry defined how many times we retry if reconciliation failed.
	MaximumRetry int
	// DeleteVMTimeout defines the timeout for the delete vm operation.
	DeleteVMTimeout time.Duration
	// BasicAuthToken is the static token to use for very basic authentication.
	BasicAuthToken string
	// TLS holds the TLS related configuration.
	TLS TLSConfig
	// DebugEndpoint is the endpoint for the debug web server. An empty string means disable the debug endpoint.
	DebugEndpoint string
	// DefaultVMProvider specifies the name of the microvm provider to use by default.
	DefaultVMProvider string
	// EnableExecAPI indicates that the MicroVMExec gRPC service should be registered.
	EnableExecAPI bool
	// EnableSSHProxyAPI indicates that the MicroVMSSHProxy gRPC service should be registered.
	EnableSSHProxyAPI bool
}

// TLSConfig holds the configuration for TLS.
type TLSConfig struct {
	// Insecure indicates if we should start the server insecurely (i.e. without TLS).
	Insecure bool
	// CertFile is the path to the certificate file to use.
	CertFile string
	// KeyFile is the path to the certificate key file to use.
	KeyFile string
	// ValidateClient indicates if the client certificates should be validated.
	ValidateClient bool
	// ClientCAFile is the path to a CA certificate file to use when validating client certificates.
	ClientCAFile string
}

// EffectiveOverlayInitrdImage returns the initrd image that boots an overlay
// root when a spec names none: OverlayInitrdImage when it is set, else the
// initrd published for buildVersion when that is a release version, else an
// empty string. A build with no release version has no published initrd,
// so it has no default until the flag is set.
func (c *Config) EffectiveOverlayInitrdImage(buildVersion string) string {
	if c.OverlayInitrdImage != "" {
		return c.OverlayInitrdImage
	}

	if !version.IsRelease(buildVersion) {
		return ""
	}

	return defaults.OverlayInitrdImage + ":" + buildVersion
}
