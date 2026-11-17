package events

import (
	"github.com/containerd/typeurl/v2"
)

func init() {
	typeurl.Register(&MicroVMSpecCreated{}, "microvm.services.api.events.microvmspeccreated")
	typeurl.Register(&MicroVMSpecUpdated{}, "microvm.services.api.events.microvmspecupdated")
	typeurl.Register(&MicroVMSpecDeleted{}, "microvm.services.api.events.microvmspecdeleted")
	typeurl.Register(&MicroVMRestored{}, "microvm.services.api.events.microvmrestored")
	typeurl.Register(&SnapshotCreated{}, "snapshot.services.api.events.snapshotcreated")
	typeurl.Register(&SnapshotDeleted{}, "snapshot.services.api.events.snapshotdeleted")
	typeurl.Register(&SnapshotUpdated{}, "snapshot.services.api.events.snapshotupdated")
}
