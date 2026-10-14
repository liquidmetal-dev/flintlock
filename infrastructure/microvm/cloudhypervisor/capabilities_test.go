package cloudhypervisor

import (
	"testing"

	g "github.com/onsi/gomega"
	"github.com/spf13/afero"

	"github.com/liquidmetal-dev/flintlock/core/models"
)

func TestProviderCapabilities(t *testing.T) {
	g.RegisterTestingT(t)

	p := New(&Config{}, nil, nil, afero.NewMemMapFs())
	g.Expect(p.Capabilities()).NotTo(g.ContainElement(models.VolumeFullCapability))
	g.Expect(p.Capabilities()).NotTo(g.ContainElement(models.VolumeOverlayCapability))

	p = New(&Config{
		VolumeCapabilities: models.Capabilities{models.VolumeFullCapability, models.VolumeOverlayCapability},
	}, nil, nil, afero.NewMemMapFs())
	g.Expect(p.Capabilities()).To(g.ContainElements(models.VolumeFullCapability, models.VolumeOverlayCapability))
	g.Expect(p.Capabilities()).To(g.ContainElement(models.VirtioFSCapability))
}
