package config

import (
	"path/filepath"
	"time"

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
	// StateRootDir is the directory to act as the root for the runtime state of flintlock.
	StateRootDir string
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
	// RepositoryStore is the name of the backing store to use for microvm spec/status
	// definitions: "containerd" or "sqlite".
	RepositoryStore string
	// SqliteDataPath is the path to the sqlite database file to use when RepositoryStore is "sqlite".
	SqliteDataPath string
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
}

// ResolvedSqliteDataPath returns SqliteDataPath if set, otherwise the
// default sqlite database path under StateRootDir.
func (c *Config) ResolvedSqliteDataPath() string {
	if c.SqliteDataPath != "" {
		return c.SqliteDataPath
	}

	return filepath.Join(c.StateRootDir, defaults.SqliteDataPath)
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
