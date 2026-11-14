package cloudhypervisor

import (
	"context"
	"testing"

	g "github.com/onsi/gomega"
	"github.com/spf13/afero"

	"github.com/liquidmetal-dev/flintlock/core/models"
)

// TestCreateSkipsWhenProcessAlive verifies the belt-and-suspenders guard in
// Create(): if a live cloud-hypervisor already owns this microvm, Create() is a
// no-op and must not touch (delete) the existing socket, which would orphan the
// running process.
func TestCreateSkipsWhenProcessAlive(t *testing.T) {
	g.RegisterTestingT(t)

	p, id, vmState := newTestProvider(t)

	// A live process owns the vm...
	g.Expect(vmState.SetPid(startLiveProcess(t))).To(g.Succeed())
	// ...and an existing socket file that must survive.
	g.Expect(afero.WriteFile(p.fs, vmState.SockPath(), []byte{}, 0o600)).To(g.Succeed())

	vmid, err := models.NewVMIDFromString(id)
	g.Expect(err).NotTo(g.HaveOccurred())

	err = p.Create(context.Background(), &models.MicroVM{ID: *vmid})
	g.Expect(err).NotTo(g.HaveOccurred())

	// Guard fired: socket left intact (ensureState never ran to remove it).
	sockExists, err := afero.Exists(p.fs, vmState.SockPath())
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(sockExists).To(g.BeTrue())
}
