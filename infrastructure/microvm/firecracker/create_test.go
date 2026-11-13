package firecracker

import (
	"os"
	"path/filepath"
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

func TestCommand_HasAPISocket(t *testing.T) {
	g.RegisterTestingT(t)

	vmid, err := models.NewVMID("test", "ns", "344780b0-6249-11ec-90d6-0242ac120003")
	g.Expect(err).NotTo(g.HaveOccurred())

	p := &fcProvider{config: &Config{FirecrackerBin: "/usr/bin/firecracker"}, fs: afero.NewMemMapFs()}
	vmState := NewState(*vmid, "/var/lib/flintlock/vm", "/run/flintlock", p.fs)

	cmd := p.command(&models.MicroVM{ID: *vmid}, vmState)

	g.Expect(cmd.Args).To(g.ContainElements("--api-sock", "/run/flintlock/344780b0-6249-11ec-90d6-0242ac120003/firecracker.sock"))
	g.Expect(cmd.Args).NotTo(g.ContainElement("--no-api"))
}

func TestEnsureState_RemovesStaleAPISocket(t *testing.T) {
	g.RegisterTestingT(t)

	vmid, err := models.NewVMID("test", "ns", "344780b0-6249-11ec-90d6-0242ac120003")
	g.Expect(err).NotTo(g.HaveOccurred())

	root := t.TempDir()
	fs := afero.NewOsFs()
	p := &fcProvider{config: &Config{}, fs: fs}
	vmState := NewState(*vmid, root+"/vm", root+"/run", fs)

	g.Expect(fs.MkdirAll(vmState.SocketRoot(), 0o700)).To(g.Succeed())

	g.Expect(afero.WriteFile(fs, vmState.APISockPath(), nil, 0o600)).To(g.Succeed())

	g.Expect(p.ensureState(vmState)).To(g.Succeed())

	exists, err := afero.Exists(fs, vmState.APISockPath())
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(exists).To(g.BeFalse())
}

func TestEnsureState_CreatesSocketLink(t *testing.T) {
	g.RegisterTestingT(t)

	vmid, err := models.NewVMID("test", "ns", "344780b0-6249-11ec-90d6-0242ac120003")
	g.Expect(err).NotTo(g.HaveOccurred())

	root := t.TempDir()
	p := &fcProvider{config: &Config{}, fs: afero.NewOsFs()}
	vmState := NewState(*vmid, root+"/vm", root+"/run", p.fs)

	g.Expect(p.ensureState(vmState)).To(g.Succeed())

	target, err := os.Readlink(vmState.SocketLinkPath())
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(target).To(g.Equal(vmState.SocketRoot()))

	info, err := os.Stat(vmState.SocketRoot())
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(info.IsDir()).To(g.BeTrue())
	g.Expect(info.Mode().Perm()).To(g.Equal(os.FileMode(socketDirPerm)))

	// The relative vsock path resolves through the link to the socket file.
	g.Expect(filepath.Join(vmState.Root(), vmState.RelativeVSockPath())).To(g.Equal(vmState.Root() + "/sock/guest-agent.vsock"))
	resolvedDir, err := filepath.EvalSymlinks(filepath.Dir(filepath.Join(vmState.Root(), vmState.RelativeVSockPath())))
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(resolvedDir).To(g.Equal(filepath.Dir(vmState.VSockPath())))

	// A second call keeps the link.
	g.Expect(p.ensureState(vmState)).To(g.Succeed())
	target, err = os.Readlink(vmState.SocketLinkPath())
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(target).To(g.Equal(vmState.SocketRoot()))
}

func TestEnsureState_ReplacesStaleSocketLink(t *testing.T) {
	g.RegisterTestingT(t)

	vmid, err := models.NewVMID("test", "ns", "344780b0-6249-11ec-90d6-0242ac120003")
	g.Expect(err).NotTo(g.HaveOccurred())

	root := t.TempDir()
	p := &fcProvider{config: &Config{}, fs: afero.NewOsFs()}
	vmState := NewState(*vmid, root+"/vm", root+"/run", p.fs)

	g.Expect(os.MkdirAll(vmState.Root(), 0o755)).To(g.Succeed())
	g.Expect(os.MkdirAll(vmState.SocketRoot(), 0o755)).To(g.Succeed())
	g.Expect(os.Symlink(root+"/elsewhere", vmState.SocketLinkPath())).To(g.Succeed())

	g.Expect(p.ensureState(vmState)).To(g.Succeed())

	target, err := os.Readlink(vmState.SocketLinkPath())
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(target).To(g.Equal(vmState.SocketRoot()))

	info, err := os.Stat(vmState.SocketRoot())
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(info.Mode().Perm()).To(g.Equal(os.FileMode(socketDirPerm)))

}

func TestEnsureState_NoSymlinkSupport(t *testing.T) {
	g.RegisterTestingT(t)

	vmid, err := models.NewVMID("test", "ns", "344780b0-6249-11ec-90d6-0242ac120003")
	g.Expect(err).NotTo(g.HaveOccurred())

	p := &fcProvider{config: &Config{}, fs: afero.NewMemMapFs()}
	vmState := NewState(*vmid, "/var/lib/flintlock/vm", "/run/flintlock", p.fs)

	g.Expect(p.ensureState(vmState)).To(g.MatchError(errSymlinkNotSupported))
}
