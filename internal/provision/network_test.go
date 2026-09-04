package provision_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/internal/provision"
)

const ipRouteShowOutput = `default via 10.0.2.2 dev eth0 proto dhcp src 10.0.2.15 metric 100
10.0.2.0/24 dev eth0 proto kernel scope link src 10.0.2.15
172.17.0.0/16 dev docker0 proto kernel scope link src 172.17.0.1 linkdown`

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
}
