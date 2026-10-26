package provision_test

import (
	"testing"

	. "github.com/onsi/gomega"
	"github.com/pelletier/go-toml"

	"github.com/liquidmetal-dev/flintlock/internal/provision"
)

func TestBuildContainerdPaths(t *testing.T) {
	g := NewWithT(t)

	prod := provision.BuildContainerdPaths(false)
	g.Expect(prod.ConfigPath).To(Equal("/etc/containerd/config.toml"))
	g.Expect(prod.RootDir).To(Equal("/var/lib/containerd"))
	g.Expect(prod.SystemdSvc).To(Equal("containerd.service"))

	dev := provision.BuildContainerdPaths(true)
	g.Expect(dev.ConfigPath).To(Equal("/etc/containerd/config-dev.toml"))
	g.Expect(dev.RootDir).To(Equal("/var/lib/containerd-dev"))
	g.Expect(dev.SystemdSvc).To(Equal("containerd-dev.service"))
	g.Expect(dev.DevMapperDir).To(Equal("/var/lib/containerd-dev/snapshotter/devmapper"))
}

func TestBuildContainerdConfig(t *testing.T) {
	g := NewWithT(t)

	paths := provision.BuildContainerdPaths(false)
	got := provision.BuildContainerdConfig(paths, "flintlock")

	g.Expect(got).To(ContainSubstring(`pool_name = "flintlock-thinpool"`))
	g.Expect(got).To(ContainSubstring(`root = "/var/lib/containerd"`))
	g.Expect(got).To(ContainSubstring(`address = "/run/containerd/containerd.sock"`))
}

func TestBuildContainerdConfigWithoutThinpool(t *testing.T) {
	g := NewWithT(t)

	paths := provision.BuildContainerdPaths(false)
	got := provision.BuildContainerdConfig(paths, "")

	g.Expect(got).NotTo(ContainSubstring("devmapper"))
	g.Expect(got).NotTo(ContainSubstring("[plugins]"))
	g.Expect(got).NotTo(ContainSubstring("pool_name"))
	g.Expect(got).To(ContainSubstring(`root = "/var/lib/containerd"`))
	g.Expect(got).To(ContainSubstring("[debug]"))
	g.Expect(toml.Unmarshal([]byte(got), &map[string]any{})).To(Succeed())
}

func TestBuildContainerdConfigWithThinpool(t *testing.T) {
	g := NewWithT(t)

	paths := provision.BuildContainerdPaths(true)
	got := provision.BuildContainerdConfig(paths, "flintlock-dev")

	g.Expect(got).To(ContainSubstring(`[plugins."io.containerd.snapshotter.v1.devmapper"]`))
	g.Expect(got).To(ContainSubstring(`pool_name = "flintlock-dev-thinpool"`))
	g.Expect(got).To(ContainSubstring(`root_path = "/var/lib/containerd-dev/snapshotter/devmapper"`))
	g.Expect(toml.Unmarshal([]byte(got), &map[string]any{})).To(Succeed())
}
