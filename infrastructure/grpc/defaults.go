package grpc

import (
	"github.com/weaveworks/flintlock/core/models"
)

func newMetadataInterface() *models.NetworkInterface {
	return &models.NetworkInterface{
		GuestDeviceName: "mmds",
		Type:            models.IfaceTypeTap,
		GuestMAC:        "AA:FF:00:00:00:01",
		StaticAddress: &models.StaticAddress{
			Address: "169.254.0.1/16",
		},
	}
}
