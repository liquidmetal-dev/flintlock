package network_test

import (
	"testing"

	g "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/infrastructure/network"
)

const fancyNewIfaceType models.IfaceType = "rainbow"

func TestNewIfaceName_tap(t *testing.T) {
	g.RegisterTestingT(t)

	name, err := network.NewIfaceName(models.IfaceTypeTap)

	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(name).To(g.MatchRegexp("^fltap[a-z0-9]{7}$"))
}

func TestNewIfaceName_macvtap(t *testing.T) {
	g.RegisterTestingT(t)

	name, err := network.NewIfaceName(models.IfaceTypeMacvtap)

	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(name).To(g.MatchRegexp("^flvtap[a-z0-9]{7}$"))
}

func TestNewIfaceName_unsupported(t *testing.T) {
	g.RegisterTestingT(t)

	name, err := network.NewIfaceName(models.IfaceTypeUnsupported)

	g.Expect(err).To(g.HaveOccurred())
	g.Expect(name).To(g.BeEmpty())
}

func TestNewIfaceName_unknownValue(t *testing.T) {
	g.RegisterTestingT(t)

	name, err := network.NewIfaceName(fancyNewIfaceType)

	g.Expect(err).To(g.HaveOccurred())
	g.Expect(name).To(g.BeEmpty())
}
