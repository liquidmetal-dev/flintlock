package cloudhypervisor

import (
	"fmt"
	"strings"
	"testing"

	g "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
)

// vmForArgs builds a minimal-but-valid microvm (kernel + root volume mounted, no
// network interfaces) so buildArgs runs to completion.
func vmForArgs(allowGuestAgent bool) *models.MicroVM {
	return &models.MicroVM{
		Spec: models.MicroVMSpec{
			VCPU:            1,
			MemoryInMb:      1024,
			AllowGuestAgent: allowGuestAgent,
			Kernel:          models.Kernel{Filename: "vmlinux"},
			RootVolume:      models.Volume{ID: "root"},
		},
		Status: models.MicroVMStatus{
			KernelMount: &models.Mount{Source: "/kernel"},
			Volumes: models.VolumeStatuses{
				"root": &models.VolumeStatus{Mount: models.Mount{Source: "/root.img"}},
			},
		},
	}
}

func TestBuildArgs_VsockWhenGuestAgentEnabled(t *testing.T) {
	g.RegisterTestingT(t)

	p, _, state := newTestProvider(t)

	args, err := p.buildArgs(vmForArgs(true), state, nil)
	g.Expect(err).NotTo(g.HaveOccurred())

	joined := strings.Join(args, " ")
	g.Expect(joined).To(g.ContainSubstring("--vsock"))
	g.Expect(joined).To(g.ContainSubstring(
		fmt.Sprintf("cid=%d,socket=%s", defaults.GuestAgentVsockCID, state.VSockPath())))
}

func TestBuildArgs_NoVsockWhenGuestAgentDisabled(t *testing.T) {
	g.RegisterTestingT(t)

	p, _, state := newTestProvider(t)

	args, err := p.buildArgs(vmForArgs(false), state, nil)
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(strings.Join(args, " ")).NotTo(g.ContainSubstring("--vsock"))
}
