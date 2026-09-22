package cloudhypervisor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	g "github.com/onsi/gomega"

	cerrors "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
)

// vmForArgs builds a minimal-but-valid microvm (kernel + root volume mounted, no
// network interfaces) so buildArgs runs to completion.
func vmForArgs(t *testing.T, allowGuestAgent bool) *models.MicroVM {
	t.Helper()

	kernelDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(kernelDir, "vmlinux"), []byte("kernel"), 0o600); err != nil {
		t.Fatal(err)
	}

	return &models.MicroVM{
		Spec: models.MicroVMSpec{
			VCPU:            1,
			MemoryInMb:      1024,
			AllowGuestAgent: allowGuestAgent,
			Kernel:          models.Kernel{Filename: "vmlinux"},
			RootVolume:      models.Volume{ID: "root"},
		},
		Status: models.MicroVMStatus{
			KernelMount: &models.Mount{Source: kernelDir},
			Volumes: models.VolumeStatuses{
				"root": &models.VolumeStatus{Mount: models.Mount{Source: "/root.img"}},
			},
		},
	}
}

func TestBuildArgs_VsockWhenGuestAgentEnabled(t *testing.T) {
	g.RegisterTestingT(t)

	p, _, state := newTestProvider(t)

	args, err := p.buildArgs(vmForArgs(t, true), state, nil)
	g.Expect(err).NotTo(g.HaveOccurred())

	joined := strings.Join(args, " ")
	g.Expect(joined).To(g.ContainSubstring("--vsock"))
	g.Expect(joined).To(g.ContainSubstring(
		fmt.Sprintf("cid=%d,socket=%s", defaults.GuestAgentVsockCID, state.VSockPath())))
}

func TestBuildArgs_NoVsockWhenGuestAgentDisabled(t *testing.T) {
	g.RegisterTestingT(t)

	p, _, state := newTestProvider(t)

	args, err := p.buildArgs(vmForArgs(t, false), state, nil)
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(strings.Join(args, " ")).NotTo(g.ContainSubstring("--vsock"))
}

func TestBuildArgs_CPUFeaturesEnabled(t *testing.T) {
	g.RegisterTestingT(t)

	p, _, state := newTestProvider(t)

	vm := vmForArgs(t, false)
	vm.Spec.CPUConfig = &models.CPUConfig{FeaturesToEnable: []string{"amx"}}

	args, err := p.buildArgs(vm, state, nil)
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(strings.Join(args, " ")).To(g.ContainSubstring("--cpus boot=1,features=amx"))
}

func TestBuildArgs_CPUFeaturesUnrecognisedIgnored(t *testing.T) {
	g.RegisterTestingT(t)

	p, _, state := newTestProvider(t)

	vm := vmForArgs(t, false)
	vm.Spec.CPUConfig = &models.CPUConfig{FeaturesToEnable: []string{"171"}}

	args, err := p.buildArgs(vm, state, nil)
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(strings.Join(args, " ")).To(g.ContainSubstring("--cpus boot=1"))
	g.Expect(strings.Join(args, " ")).NotTo(g.ContainSubstring("features"))
}

func TestBuildArgs_CPUConfigNil(t *testing.T) {
	g.RegisterTestingT(t)

	p, _, state := newTestProvider(t)

	args, err := p.buildArgs(vmForArgs(t, false), state, nil)
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(strings.Join(args, " ")).To(g.ContainSubstring("--cpus boot=1"))
	g.Expect(strings.Join(args, " ")).NotTo(g.ContainSubstring("features"))
}

func TestBuildArgs_KernelPathWithinMount(t *testing.T) {
	g.RegisterTestingT(t)

	p, _, state := newTestProvider(t)

	vm := vmForArgs(t, false)

	args, err := p.buildArgs(vm, state, nil)
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(strings.Join(args, " ")).To(g.ContainSubstring(
		"--kernel " + filepath.Join(vm.Status.KernelMount.Source, "vmlinux")))
}

func TestBuildArgs_KernelPathTraversalRejected(t *testing.T) {
	g.RegisterTestingT(t)

	p, _, state := newTestProvider(t)

	vm := vmForArgs(t, false)
	vm.Spec.Kernel.Filename = "../../../../../../etc/passwd"

	_, err := p.buildArgs(vm, state, nil)
	g.Expect(err).To(g.MatchError(cerrors.ErrInvalidImageFilePath))
}

func TestBuildArgs_EmptyKernelMountRejected(t *testing.T) {
	g.RegisterTestingT(t)

	p, _, state := newTestProvider(t)

	vm := vmForArgs(t, false)
	vm.Status.KernelMount.Source = ""

	_, err := p.buildArgs(vm, state, nil)
	g.Expect(err).To(g.MatchError(cerrors.ErrInvalidImageFilePath))
}
