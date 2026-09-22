package cloudhypervisor

import (
	"strings"
	"testing"

	g "github.com/onsi/gomega"
	"github.com/spf13/afero"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
)

const testULID = "01J9Z3V7C8X4K2M5N6P7Q8R9ST"

func TestStateSocketPathsIndependentOfName(t *testing.T) {
	g.RegisterTestingT(t)

	vmid, err := models.NewVMID(strings.Repeat("n", 63), strings.Repeat("s", 63), testULID)
	g.Expect(err).NotTo(g.HaveOccurred())

	state := NewState(*vmid, defaults.StateRootDir+"/vm", defaults.SocketRootDir, afero.NewMemMapFs())

	g.Expect(state.SocketRoot()).To(g.Equal(defaults.SocketRootDir + "/" + testULID))

	for _, path := range []string{state.SockPath(), state.VSockPath()} {
		g.Expect(path).To(g.HavePrefix(state.SocketRoot() + "/"))
		g.Expect(len(path) + models.SocketSuffixReserve).To(g.BeNumerically("<=", models.MaxUnixSocketPathLength))
	}
}

func TestStateResolveSockPathFallsBackToLegacy(t *testing.T) {
	g.RegisterTestingT(t)

	vmid, err := models.NewVMID("name", "ns", testULID)
	g.Expect(err).NotTo(g.HaveOccurred())

	fs := afero.NewMemMapFs()
	state := NewState(*vmid, "/var/lib/flintlock/vm", "/run/flintlock", fs)

	g.Expect(state.ResolveSockPath()).To(g.Equal(state.SockPath()))

	g.Expect(afero.WriteFile(fs, state.legacySockPath(), nil, 0o600)).To(g.Succeed())
	g.Expect(state.ResolveSockPath()).To(g.Equal(state.legacySockPath()))

	g.Expect(afero.WriteFile(fs, state.SockPath(), nil, 0o600)).To(g.Succeed())
	g.Expect(state.ResolveSockPath()).To(g.Equal(state.SockPath()))
}

func TestEnsureStateRemovesLegacySockets(t *testing.T) {
	g.RegisterTestingT(t)

	vmid, err := models.NewVMID("name", "ns", testULID)
	g.Expect(err).NotTo(g.HaveOccurred())

	fs := afero.NewMemMapFs()
	p := &provider{config: &Config{StateRoot: "/var/lib/flintlock/vm", SocketDir: "/run/flintlock"}, fs: fs}
	state := NewState(*vmid, p.config.StateRoot, p.config.SocketDir, fs)

	for _, path := range []string{state.legacySockPath(), state.legacyVSockPath()} {
		g.Expect(afero.WriteFile(fs, path, nil, 0o600)).To(g.Succeed())
	}

	g.Expect(p.ensureState(state)).To(g.Succeed())

	for _, path := range []string{state.legacySockPath(), state.legacyVSockPath()} {
		g.Expect(afero.Exists(fs, path)).To(g.BeFalse())
	}
}

func TestSocketNamesFitMaxSocketNameLength(t *testing.T) {
	g.RegisterTestingT(t)

	for _, name := range []string{socketFileName, defaults.GuestAgentVsockName} {
		g.Expect(len(name)).To(g.BeNumerically("<=", models.MaxSocketNameLength))
	}
}
