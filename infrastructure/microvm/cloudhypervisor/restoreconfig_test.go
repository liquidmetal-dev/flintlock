package cloudhypervisor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	g "github.com/onsi/gomega"
)

func fixturePaths() restoreConfigPaths {
	return restoreConfigPaths{
		Disks: []string{
			"/dev/mapper/flintlock-new-root",
			"/var/lib/flintlock/vm/new/cloud-init.img",
			"/dev/mapper/flintlock-new-data",
		},
		Taps:       []string{"fltapnew0", "fltapnew1"},
		FSSockets:  []string{"/run/flintlock/new/virtiofs.sock"},
		VSock:      "/run/flintlock/new/flintlock-agent.vsock",
		StateRoot:  "/var/lib/flintlock/vm/new",
		SocketRoot: "/run/flintlock/new",
	}
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", "snapshot", name))
	g.Expect(err).NotTo(g.HaveOccurred())

	return data
}

func TestRewriteRestoreConfig(t *testing.T) {
	g.RegisterTestingT(t)

	got, err := rewriteRestoreConfig(readFixture(t, "config.json"), fixturePaths())
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(got).To(g.MatchJSON(readFixture(t, "config.restored.json")))
}

func TestRewriteRestoreConfig_KeepsNumbers(t *testing.T) {
	g.RegisterTestingT(t)

	in := `{"memory":{"size":18446744073709551615},"disks":null,"net":null,"vsock":null}`

	got, err := rewriteRestoreConfig([]byte(in), restoreConfigPaths{})
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(string(got)).To(g.ContainSubstring(`"size":18446744073709551615`))
}

func TestRewriteRestoreConfig_ConsoleSocket(t *testing.T) {
	g.RegisterTestingT(t)

	in := `{"console":{"file":null,"mode":"Socket","socket":"/run/flintlock/old/console.sock"}}`

	paths := restoreConfigPaths{StateRoot: "/var/lib/flintlock/vm/new", SocketRoot: "/run/flintlock/new"}

	got, err := rewriteRestoreConfig([]byte(in), paths)
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(got).To(g.MatchJSON(`{"console":{"file":null,"mode":"Socket","socket":"/run/flintlock/new/console.sock"}}`))
}

func TestRewriteRestoreConfig_Mismatch(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(paths *restoreConfigPaths)
	}{
		{name: "fewer disks", mutate: func(p *restoreConfigPaths) { p.Disks = p.Disks[:2] }},
		{name: "more taps", mutate: func(p *restoreConfigPaths) { p.Taps = append(p.Taps, "fltapnew2") }},
		{name: "no virtiofs", mutate: func(p *restoreConfigPaths) { p.FSSockets = nil }},
		{name: "no vsock", mutate: func(p *restoreConfigPaths) { p.VSock = "" }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g.RegisterTestingT(t)

			paths := fixturePaths()
			tc.mutate(&paths)

			_, err := rewriteRestoreConfig(readFixture(t, "config.json"), paths)
			g.Expect(err).To(g.MatchError(errSnapshotDeviceMismatch))
		})
	}
}

func TestRewriteRestoreConfig_VSockInMicroVMOnly(t *testing.T) {
	g.RegisterTestingT(t)

	var config map[string]json.RawMessage
	g.Expect(json.Unmarshal(readFixture(t, "config.json"), &config)).To(g.Succeed())
	config["vsock"] = json.RawMessage("null")

	data, err := json.Marshal(config)
	g.Expect(err).NotTo(g.HaveOccurred())

	_, err = rewriteRestoreConfig(data, fixturePaths())
	g.Expect(err).To(g.MatchError(errSnapshotDeviceMismatch))
}

func TestRewriteRestoreConfig_InvalidJSON(t *testing.T) {
	g.RegisterTestingT(t)

	_, err := rewriteRestoreConfig([]byte("not json"), fixturePaths())
	g.Expect(err).To(g.HaveOccurred())
}
