package network_test

import "github.com/liquidmetal-dev/flintlock/core/models"

const (
	vmName                    = "testvm"
	nsName                    = "testns"
	vmUID                     = "testuid"
	defaultMACAddress         = "AA:BB:CC:DD:EE:FF"
	reverseMACAddress         = "FF:EE:DD:CC:BB:AA"
	expectedTapDeviceName     = "testns_testvm_tap"
	expectedMacvtapDeviceName = "testns_testvm_vtap"
	defaultEthDevice          = "eth0"
)

func fullNetworkInterface() (*models.NetworkInterface, *models.NetworkInterfaceStatus) {
	iface := &models.NetworkInterface{
		GuestDeviceName:       defaultEthDevice,
		AllowMetadataRequests: true,
		GuestMAC:              defaultMACAddress,
	}
	status := &models.NetworkInterfaceStatus{
		HostDeviceName: expectedTapDeviceName,
		Index:          0,
		MACAddress:     defaultMACAddress,
	}

	return iface, status
}
