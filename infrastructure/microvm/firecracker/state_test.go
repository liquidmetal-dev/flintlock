package firecracker_test

import (
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
