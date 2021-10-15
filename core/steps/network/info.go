package network

import (
	"fmt"

	"github.com/weaveworks/reignite/core/models"
)

func getDeviceName(vmid *models.VMID, iface *models.NetworkInterface) string {
	if iface.Type == models.IfaceTypeMacvtap {
		return fmt.Sprintf(macvtapFormat, vmid.Namespace(), vmid.Name())
	}

	return fmt.Sprintf(tapFormat, vmid.Namespace(), vmid.Name())
}
