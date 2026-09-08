package defaults

import (
	"time"
)

const (
	// Domain is the reverse order domain name to use.
	Domain = "dev.liquidmetal.flintlockd"

	// ContainerdNamespace is the name of the namespace to use with containerd.
	ContainerdNamespace = "flintlock"

	// ContainerdSocket is the defaults path for the containerd socket.
	ContainerdSocket = "/run/containerd/containerd.sock"

	// ContainerdVolumeSnapshotter is the name of the snapshotter used for volumes.
	ContainerdVolumeSnapshotter = "devmapper"

	// ContainerdKernelSnapshotter is the name of the default snapshotter to use for kernek/initrd.
	ContainerdKernelSnapshotter = "native"

	// FirecrackerBin is the name of the firecracker binary.
	FirecrackerBin = "firecracker"

	// FirecrackerDetach is the default for the flag to indicates with the child firecracker
	// processes should be run detached.
	FirecrackerDetach = true

	// CloudHypervisorBin is the name of the Cloud Hypervisor binary.
	CloudHypervisorBin = "cloud-hypervisor-static"

	// VirtioFSBin is the name of the virtiofsd binary.
	VirtioFSBin = "/usr/libexec/virtiofsd"

	// CloudHypervisorDetach is the default for the flag to indicates with the child cloud-hypervisor
	// processes should be run detached.
	CloudHypervisorDetach = true

	// ConfigurationDir is the default configuration directory.
	ConfigurationDir = "/etc/opt/flintlockd"

	// StateRootDir is the default directory to use for state information.
	StateRootDir = "/var/lib/flintlock"

	// GRPCEndpoint is the endpoint for the gRPC server.
	GRPCAPIEndpoint = "localhost:9090"

	// HTTPAPIEndpoint is the endpoint for the HHTP proxy for the gRPC service..
	HTTPAPIEndpoint = "localhost:8090"

	// TopicMicroVMEvents is the topic name to use for microvm events.
	TopicMicroVMEvents = "/microvm"

	// MicroVMNamespace is the default namespace to use for microvms.
	MicroVMNamespace = "default"

	// ResyncPeriod is the default resync period duration.
	ResyncPeriod time.Duration = 10 * time.Minute

	// DeleteVMTimeout is the default timeout for deleting a microvm.
	DeleteVMTimeout time.Duration = 10 * time.Second

	// DataDirPerm is the permissions to use for data folders.
	DataDirPerm = 0o755

	// DataFilePerm is the permissions to use for data files.
	DataFilePerm = 0o644

	// MaximumRetry is the default value how many times we retry failed reconciliation.
	MaximumRetry = 10

	// Namespace is the default MicroVM namespace if one is not provided by the user.
	Namespace = "default"

	// VCPU is the default number if VCPUs for a MicroVM if one is not provided by the user.
	VCPU = 2

	// MemoryInMb is the default amount of RAM for a MicroVM if one is not provided by the user.
	MemoryInMb = 1024

	// GuestAgentVsockCID is the guest context id (CID) used for the guest-agent vsock device.
	// The host is always CID 2; guests start at 3.
	GuestAgentVsockCID = 3

	// GuestAgentVsockName is the host unix-socket filename for the guest-agent vsock device.
	GuestAgentVsockName = "guest-agent.vsock"

	// GuestAgentControlPort is the guest-agent's control-channel vsock port (exec/ping/info).
	GuestAgentControlPort = 1024

	// GuestAgentSSHPort is the guest-agent's ssh-proxy vsock port.
	GuestAgentSSHPort = 1025

	// ExecSessionIdleGrace is added on top of an exec request's own
	// TimeoutSec to get the idle deadline an exec session starts with,
	// before its guest-agent has proven (by sending a FrameHeartbeat) that
	// it supports heartbeat-based liveness: the guest-agent enforces
	// TimeoutSec itself and is expected to respond by then, so this is
	// just a buffer for it to report that outcome (plus network latency)
	// before the host gives up on a wedged connection.
	ExecSessionIdleGrace time.Duration = 30 * time.Second

	// ExecSessionUnboundedIdleCeiling is the idle read deadline an exec
	// session with no TimeoutSec (0, "unbounded") starts with, before its
	// guest-agent has proven it supports heartbeat-based liveness. There's
	// no declared budget to derive a deadline from in that case, and (for
	// a guest-agent predating heartbeats) no protocol-level way to tell a
	// healthy quiet command from a wedged connection, so this is a
	// last-resort circuit breaker rather than a liveness check: sized
	// generously so it's not expected to trip a real quiet workload, while
	// still guaranteeing the session eventually gives up on a dead
	// transport instead of blocking forever.
	ExecSessionUnboundedIdleCeiling time.Duration = 15 * time.Minute

	// ExecSessionIdleTimeout is the idle read deadline an exec session
	// switches to once its guest-agent has sent at least one
	// FrameHeartbeat, proving it supports heartbeat-based liveness
	// (guest-agent v0.4.0+, which emits one on a fixed 5s ticker for the
	// duration of a running command, independent of the command's own
	// declared timeout or output activity). This is a real liveness
	// signal rather than a proxy for one: sized to 3x the heartbeat
	// interval to tolerate one dropped/delayed heartbeat before giving up
	// on a wedged connection. Applies uniformly from that point on,
	// regardless of the exec request's TimeoutSec.
	//
	// A guest-agent that never sends a heartbeat (pre-v0.4.0) keeps the
	// TimeoutSec-derived deadline (ExecSessionIdleGrace /
	// ExecSessionUnboundedIdleCeiling above) for the life of the session.
	ExecSessionIdleTimeout time.Duration = 15 * time.Second
)
