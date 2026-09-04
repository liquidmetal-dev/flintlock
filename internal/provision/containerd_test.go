package provision_test

import (
	"testing"

	. "github.com/onsi/gomega"

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
