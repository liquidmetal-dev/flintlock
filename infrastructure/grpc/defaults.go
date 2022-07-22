package grpc

import (
	"github.com/weaveworks-liquidmetal/flintlock/core/models"
)

func newMetadataInterface() *models.NetworkInterface {
	return &models.NetworkInterface{
		GuestDeviceName:       "eth0",
		Type:                  models.IfaceTypeTap,
		AllowMetadataRequests: true,
		GuestMAC:              "AA:FF:00:00:00:01",
		StaticAddress: &models.StaticAddress{
			Address: "169.254.0.1/16",
		},
	}
}
