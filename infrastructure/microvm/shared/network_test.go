package shared_test

import (
	"encoding/base64"
	"testing"

	g "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"

	"github.com/liquidmetal-dev/flintlock/client/cloudinit/network"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/infrastructure/microvm/shared"
)

func vmWithMetadataAndDataInterfaces() *models.MicroVM {
	return &models.MicroVM{
		Spec: models.MicroVMSpec{
			NetworkInterfaces: []models.NetworkInterface{
				{
					GuestDeviceName:       "eth0",
					Type:                  models.IfaceTypeTap,
					AllowMetadataRequests: true,
					GuestMAC:              "AA:FF:00:00:00:01",
					StaticAddress:         &models.StaticAddress{Address: "169.254.0.1/16"},
				},
				{
					GuestDeviceName: "eth1",
					Type:            models.IfaceTypeTap,
					GuestMAC:        "AA:FF:00:00:00:02",
					StaticAddress:   &models.StaticAddress{Address: "10.0.0.2/24"},
				},
			},
		},
		Status: models.MicroVMStatus{
			NetworkInterfaces: models.NetworkInterfaceStatuses{
				"eth0": &models.NetworkInterfaceStatus{},
				"eth1": &models.NetworkInterfaceStatus{},
			},
		},
	}
}

func decodeNetworkConfig(t *testing.T, encoded string) network.Network {
	t.Helper()

	raw, err := base64.StdEncoding.DecodeString(encoded)
	g.Expect(err).NotTo(g.HaveOccurred())

	var netConf network.Network
	g.Expect(yaml.Unmarshal(raw, &netConf)).To(g.Succeed())

	return netConf
}

func TestGenerateNetworkConfig_ExcludesInterface(t *testing.T) {
	g.RegisterTestingT(t)

	encoded, err := shared.GenerateNetworkConfig(vmWithMetadataAndDataInterfaces(), "eth0")
	g.Expect(err).NotTo(g.HaveOccurred())

	netConf := decodeNetworkConfig(t, encoded)
	g.Expect(netConf.Ethernet).To(g.HaveLen(1))
	g.Expect(netConf.Ethernet).To(g.HaveKey("eth1"))
}

func TestGenerateNetworkConfig_NoExclude(t *testing.T) {
	g.RegisterTestingT(t)

	encoded, err := shared.GenerateNetworkConfig(vmWithMetadataAndDataInterfaces())
	g.Expect(err).NotTo(g.HaveOccurred())

	netConf := decodeNetworkConfig(t, encoded)
	g.Expect(netConf.Ethernet).To(g.HaveLen(2))
	g.Expect(netConf.Ethernet["eth0"].Addresses).To(g.Equal([]string{"169.254.0.1/16"}))
	g.Expect(netConf.Ethernet).To(g.HaveKey("eth1"))
}
