package firecracker_test

import (
	"encoding/base64"
	"strings"
	"testing"

	g "github.com/onsi/gomega"
	"github.com/spf13/afero"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/infrastructure/microvm/firecracker"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
)

func TestStateVSockPathIndependentOfName(t *testing.T) {
	g.RegisterTestingT(t)

	uid := "01J9Z3V7C8X4K2M5N6P7Q8R9ST"
	vmid, err := models.NewVMID(strings.Repeat("n", 63), strings.Repeat("s", 63), uid)
	g.Expect(err).NotTo(g.HaveOccurred())

	state := firecracker.NewState(*vmid, defaults.StateRootDir+"/vm", defaults.SocketRootDir, afero.NewMemMapFs())

	g.Expect(state.VSockPath()).To(g.Equal(defaults.SocketRootDir + "/" + uid + "/" + defaults.GuestAgentVsockName))
	g.Expect(len(state.VSockPath()) + models.SocketSuffixReserve).To(g.BeNumerically("<=", models.MaxUnixSocketPathLength))
}

func TestStateAPISockPath(t *testing.T) {
	g.RegisterTestingT(t)

	uid := "01J9Z3V7C8X4K2M5N6P7Q8R9ST"
	vmid, err := models.NewVMID(strings.Repeat("n", 63), strings.Repeat("s", 63), uid)
	g.Expect(err).NotTo(g.HaveOccurred())

	state := firecracker.NewState(*vmid, defaults.StateRootDir+"/vm", defaults.SocketRootDir, afero.NewMemMapFs())

	g.Expect(state.APISockPath()).To(g.Equal(defaults.SocketRootDir + "/" + uid + "/firecracker.sock"))
	g.Expect(len(state.APISockPath())).To(g.BeNumerically("<=", models.MaxUnixSocketPathLength))
}

func TestStateMetadataReadsMetadataFile(t *testing.T) {
	g.RegisterTestingT(t)

	vmid, err := models.NewVMID("test", "ns", "344780b0-6249-11ec-90d6-0242ac120003")
	g.Expect(err).NotTo(g.HaveOccurred())

	state := firecracker.NewState(*vmid, "/var/lib/flintlock/vm", "/run/flintlock", afero.NewMemMapFs())

	g.Expect(state.SetConfig(&firecracker.VmmConfig{})).To(g.Succeed())
	g.Expect(state.SetMetadata(&firecracker.Metadata{
		Latest: map[string]string{
			"user-data": base64.StdEncoding.EncodeToString([]byte("#cloud-config")),
			"plain":     "not base64!",
		},
	})).To(g.Succeed())

	meta, err := state.Metadata()
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(meta.Latest).To(g.Equal(map[string]string{
		"user-data": "#cloud-config",
		"plain":     "not base64!",
	}))
}
