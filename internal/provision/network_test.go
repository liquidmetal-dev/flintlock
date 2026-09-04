package provision_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/internal/provision"
)

const ipRouteShowOutput = `default via 10.0.2.2 dev eth0 proto dhcp src 10.0.2.15 metric 100
10.0.2.0/24 dev eth0 proto kernel scope link src 10.0.2.15
172.17.0.0/16 dev docker0 proto kernel scope link src 172.17.0.1 linkdown
172.18.0.0/16 dev eth10 proto kernel scope link src 172.18.0.5`

func TestParseDefaultInterface(t *testing.T) {
	g := NewWithT(t)

	g.Expect(provision.ParseDefaultInterface(ipRouteShowOutput)).To(Equal("eth0"))
	g.Expect(provision.ParseDefaultInterface("")).To(Equal(""))
}

func TestParseAddressForInterface(t *testing.T) {
	g := NewWithT(t)

	g.Expect(provision.ParseAddressForInterface(ipRouteShowOutput, "eth0")).To(Equal("10.0.2.15"))
	g.Expect(provision.ParseAddressForInterface(ipRouteShowOutput, "docker0")).To(Equal("172.17.0.1"))
	g.Expect(provision.ParseAddressForInterface(ipRouteShowOutput, "unknown0")).To(Equal(""))

	// An empty iface must never match every line.
	g.Expect(provision.ParseAddressForInterface(ipRouteShowOutput, "")).To(Equal(""))

	// "eth1" must not substring-match "eth10".
	g.Expect(provision.ParseAddressForInterface(ipRouteShowOutput, "eth1")).To(Equal(""))
	g.Expect(provision.ParseAddressForInterface(ipRouteShowOutput, "eth10")).To(Equal("172.18.0.5"))
}
