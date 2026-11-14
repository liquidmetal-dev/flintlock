package provision_test

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/internal/provision"
)

func TestBuildFlintlockdSettings_ParentIface(t *testing.T) {
	g := NewWithT(t)

	settings := provision.BuildFlintlockdSettings(provision.FlintlockdSettings{
		ContainerdSocket: "/run/containerd/containerd.sock",
		Address:          "10.0.0.5",
		Port:             "9090",
		ParentIface:      "eth0",
		Insecure:         true,
	})

	g.Expect(settings).To(Equal(map[string]string{
		"containerd-socket": "/run/containerd/containerd.sock",
		"grpc-endpoint":     "10.0.0.5:9090",
		"verbosity":         "9",
		"insecure":          "true",
		"parent-iface":      "eth0",
	}))
}

func TestBuildFlintlockdSettings_Bridge(t *testing.T) {
	g := NewWithT(t)

	settings := provision.BuildFlintlockdSettings(provision.FlintlockdSettings{
		ContainerdSocket: "/run/containerd/containerd.sock",
		Address:          "10.0.0.5",
		Port:             "9090",
		ParentIface:      "eth0",
		BridgeName:       "br0",
	})

	g.Expect(settings).To(HaveKeyWithValue("bridge-name", "br0"))
	g.Expect(settings).NotTo(HaveKey("parent-iface"))
}

func TestMergeConfigFile(t *testing.T) {
	g := NewWithT(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	g.Expect(os.WriteFile(path, []byte("verbosity: 3\nlog-level : debug\nnot-a-valid-line\n"), 0o600)).To(Succeed())

	settings := map[string]string{
		"verbosity": "9",
		"insecure":  "false",
	}

	g.Expect(provision.MergeConfigFile(settings, path)).To(Succeed())

	g.Expect(settings).To(Equal(map[string]string{
		"verbosity": "3",
		"insecure":  "false",
		"log-level": "debug",
	}))
}

func TestBuildFlintlockdConfig(t *testing.T) {
	g := NewWithT(t)

	got := provision.BuildFlintlockdConfig(map[string]string{
		"b": "2",
		"a": "1",
	})

	g.Expect(got).To(Equal("---\na: 1\nb: 2\n"))
}
