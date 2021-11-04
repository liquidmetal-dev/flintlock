package config

import (
	"time"

	"github.com/weaveworks/flintlock/pkg/log"
)

// Config represents the flintlockd configuration.
type Config struct {
	// ConfigFilePath is the path to the shared configuration file.
	ConfigFilePath string
	// Logging contains the logging related config.
	Logging log.Config
	// GRPCEndpoint is the endpoint for the gRPC server.
	GRPCAPIEndpoint string
	// HTTPAPIEndpoint is the endpoint for the HHTP proxy for the gRPC service..
	HTTPAPIEndpoint string
	// FirecrackerBin is the firecracker binary to use.
	FirecrackerBin string
	// FirecrackerDetach indicates if the child firecracker processes should be detached from their parent.
	FirecrackerDetach bool
	// FirecrackerUseAPI indicates that we should configure the microvm using the api and not a config file.
	FirecrackerUseAPI bool
	// StateRootDir is the directory to act as the root for the runtime state of flintlock.
	StateRootDir string
	// ParentIface is the name of the network interface to use for the parent in macvtap interfaces.
	ParentIface string
	// CtrKernelSnapshotter is the name of the containerd snapshotter to use for kernel images.
	CtrKernelSnapshotter string
	// CtrVolumeSnapshotter is the name of the containerd snapshotter to use for volume (inc initrd) images.
	CtrVolumeSnapshotter string
	// CtrSocketPath is the path to the containerd socket.
	CtrSocketPath string
	// CtrNamespace is the default containerd namespace to use
	CtrNamespace string
	// DisableReconcile is used to stop the reconcile part from running.
	DisableReconcile bool
	// DisableAPI is used to disable the api server.
	DisableAPI bool
	// ResyncPeriod defines the period when we should do a reconcile of the microvms (even if there are no events).
	ResyncPeriod time.Duration
	// MaximumRetry defined how many times we retry if reconciliation failed.
	MaximumRetry int
}
