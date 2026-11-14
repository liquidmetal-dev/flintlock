package firecracker

import (
	"testing"

	g "github.com/onsi/gomega"
	"github.com/spf13/afero"

	"github.com/liquidmetal-dev/flintlock/core/models"
)

func TestCommand_RunsInStateDirectory(t *testing.T) {
	g.RegisterTestingT(t)

	vmid, err := models.NewVMID("test", "ns", "344780b0-6249-11ec-90d6-0242ac120003")
	g.Expect(err).NotTo(g.HaveOccurred())

	p := &fcProvider{config: &Config{FirecrackerBin: "/usr/bin/firecracker"}, fs: afero.NewMemMapFs()}
	vmState := NewState(*vmid, "/var/lib/flintlock/vm", "/run/flintlock", p.fs)

	cmd := p.command(&models.MicroVM{ID: *vmid}, vmState)

	g.Expect(cmd.Dir).To(g.Equal("/var/lib/flintlock/vm/ns/test/344780b0-6249-11ec-90d6-0242ac120003"))
	g.Expect(cmd.Args).To(g.ContainElements("--config-file", vmState.ConfigPath(), "--metadata", vmState.MetadataPath()))
}
