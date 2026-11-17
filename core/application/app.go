package application

import (
	"sync"
	"time"

	"github.com/liquidmetal-dev/flintlock/core/ports"
)

// App is the interface for the core application. In the future this could be split
// into separate command, query and reconcile services.
type App interface {
	ports.MicroVMCommandUseCases
	ports.MicroVMQueryUseCases
	ports.ReconcileMicroVMsUseCase
	ports.SnapshotCommandUseCases
	ports.SnapshotQueryUseCases
	ports.SnapshotCaptureUseCase
	ports.ReconcileSnapshotsUseCase
}

func New(cfg *Config, ports *ports.Collection) App {
	return &app{
		cfg:     cfg,
		ports:   ports,
		vmLocks: NewVMLocks(),

		lockedRequeueDelay: lockedRequeueDelay,
	}
}

type app struct {
	cfg   *Config
	ports *ports.Collection

	// snapshotMu makes the in-progress check and the create of a snapshot one step,
	// so two requests for the same microvm cannot both pass the check. A pause and a
	// resume hold it too, so that they do not change a microvm while a snapshot of it
	// is requested, and two of them do not interleave.
	snapshotMu sync.Mutex

	// vmLocks keeps a snapshot capture and other work on the same microvm apart.
	vmLocks *VMLocks

	// lockedRequeueDelay is how long a reconcile of a locked microvm waits before
	// it is tried again.
	lockedRequeueDelay time.Duration
}

type Config struct {
	RootStateDir    string
	SocketDir       string
	MaximumRetry    int
	DefaultProvider string
	// OverlayInitrdImage is the OCI image of the initrd that boots an overlay
	// root when the spec names none. An empty value means the daemon has no
	// default, and a spec that needs one is rejected.
	OverlayInitrdImage string
	// VMMStartTimeout is how long the create plan waits for the VMM to run.
	VMMStartTimeout time.Duration
	// GuestAgentReadyTimeout is how long the create plan waits for the guest-agent to answer a ping.
	GuestAgentReadyTimeout time.Duration
	// SnapshotMaxPause is the longest time a snapshot capture keeps the source
	// microvm paused. Zero selects defaults.SnapshotMaxPause.
	SnapshotMaxPause time.Duration
	// StartedAt is when this run of flintlockd started. A snapshot created before it
	// was left in progress by an earlier run. Zero means every snapshot in progress was.
	StartedAt time.Time
}
