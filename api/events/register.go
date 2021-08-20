package events

import (
	"github.com/containerd/typeurl"
)

func init() {
	typeurl.Register(&MicroVMSpecCreated{}, "microvm.services.api.events.microvmspeccreated")
	typeurl.Register(&MicroVMSpecUpdated{}, "microvm.services.api.events.microvmspecupdated")
	typeurl.Register(&MicroVMSpecDeleted{}, "microvm.services.api.events.microvmspecdeleted")
}
