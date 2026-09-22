package firecracker_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	g "github.com/onsi/gomega"
	"github.com/spf13/afero"

	"github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/infrastructure/microvm/firecracker"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
)

func testVSockState(t *testing.T) firecracker.State {
	t.Helper()

	vmid, err := models.NewVMID("test", "ns", "344780b0-6249-11ec-90d6-0242ac120003")
	g.Expect(err).NotTo(g.HaveOccurred())

	return firecracker.NewState(*vmid, "/var/lib/flintlock", "/run/flintlock", afero.NewMemMapFs())
}

func TestWithVsock_Enabled(t *testing.T) {
	g.RegisterTestingT(t)

	state := testVSockState(t)
	vm := &models.MicroVM{Spec: models.MicroVMSpec{AllowGuestAgent: true}}

	cfg, err := firecracker.CreateConfig(firecracker.WithVsock(vm, state))
	g.Expect(err).NotTo(g.HaveOccurred())

	g.Expect(cfg.VsockDevice).NotTo(g.BeNil())
	g.Expect(cfg.VsockDevice.GuestCID).To(g.Equal(int64(defaults.GuestAgentVsockCID)))
	g.Expect(cfg.VsockDevice.UDSPath).To(g.Equal(state.VSockPath()))
	g.Expect(strings.HasSuffix(cfg.VsockDevice.UDSPath, defaults.GuestAgentVsockName)).To(g.BeTrue())
}

func TestWithVsock_Disabled(t *testing.T) {
	g.RegisterTestingT(t)

	state := testVSockState(t)
	vm := &models.MicroVM{Spec: models.MicroVMSpec{AllowGuestAgent: false}}

	cfg, err := firecracker.CreateConfig(firecracker.WithVsock(vm, state))
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(cfg.VsockDevice).To(g.BeNil())
}

// vmForMicroVM builds a minimal-but-valid microvm (kernel + root volume mounted, no
// network interfaces) so WithMicroVM runs to completion.
func vmForMicroVM(t *testing.T, cpuConfig *models.CPUConfig) *models.MicroVM {
	t.Helper()

	kernelDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(kernelDir, "vmlinux"), []byte("kernel"), 0o600); err != nil {
		t.Fatal(err)
	}

	return &models.MicroVM{
		Spec: models.MicroVMSpec{
			VCPU:       1,
			MemoryInMb: 1024,
			Kernel:     models.Kernel{Filename: "vmlinux"},
			RootVolume: models.Volume{ID: "root"},
			CPUConfig:  cpuConfig,
		},
		Status: models.MicroVMStatus{
			KernelMount: &models.Mount{Source: kernelDir},
			Volumes: models.VolumeStatuses{
				"root": &models.VolumeStatus{Mount: models.Mount{Source: "/root.img"}},
			},
		},
	}
}

func TestWithMicroVM_CPUConfig_EnableAndDisable(t *testing.T) {
	g.RegisterTestingT(t)

	vm := vmForMicroVM(t, &models.CPUConfig{
		FeaturesToEnable:         []string{"171"},
		KVMCapabilitiesToDisable: []string{"56"},
	})

	cfg, err := firecracker.CreateConfig(firecracker.WithMicroVM(vm))
	g.Expect(err).NotTo(g.HaveOccurred())

	g.Expect(cfg.CPUConfig).NotTo(g.BeNil())
	g.Expect(cfg.CPUConfig.KvmCapabilities).To(g.ConsistOf("171", "!56"))
}

func TestWithMicroVM_CPUConfig_Nil(t *testing.T) {
	g.RegisterTestingT(t)

	vm := vmForMicroVM(t, nil)

	cfg, err := firecracker.CreateConfig(firecracker.WithMicroVM(vm))
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(cfg.CPUConfig).To(g.BeNil())
}

func TestWithMicroVM_CPUConfig_Empty(t *testing.T) {
	g.RegisterTestingT(t)

	vm := vmForMicroVM(t, &models.CPUConfig{})

	cfg, err := firecracker.CreateConfig(firecracker.WithMicroVM(vm))
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(cfg.CPUConfig).To(g.BeNil())
}

func TestWithMicroVM_BootSourcePathsWithinMount(t *testing.T) {
	g.RegisterTestingT(t)

	vm := vmForMicroVM(t, nil)

	initrdDir := t.TempDir()
	g.Expect(os.WriteFile(filepath.Join(initrdDir, "initrd.img"), []byte("initrd"), 0o600)).To(g.Succeed())
	vm.Spec.Initrd = &models.Initrd{Filename: "initrd.img"}
	vm.Status.InitrdMount = &models.Mount{Source: initrdDir}

	cfg, err := firecracker.CreateConfig(firecracker.WithMicroVM(vm))
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(cfg.BootSource.KernelImagePage).To(g.Equal(filepath.Join(vm.Status.KernelMount.Source, "vmlinux")))
	g.Expect(cfg.BootSource.InitrdPath).To(g.HaveValue(g.Equal(filepath.Join(initrdDir, "initrd.img"))))
}

func TestWithMicroVM_KernelPathTraversalRejected(t *testing.T) {
	g.RegisterTestingT(t)

	vm := vmForMicroVM(t, nil)
	vm.Spec.Kernel.Filename = "../../../../../../etc/passwd"

	_, err := firecracker.CreateConfig(firecracker.WithMicroVM(vm))
	g.Expect(err).To(g.MatchError(errors.ErrInvalidImageFilePath))
}

func TestWithMicroVM_InitrdPathTraversalRejected(t *testing.T) {
	g.RegisterTestingT(t)

	vm := vmForMicroVM(t, nil)
	vm.Spec.Initrd = &models.Initrd{Filename: "../../../../../../etc/passwd"}
	vm.Status.InitrdMount = &models.Mount{Source: t.TempDir()}

	_, err := firecracker.CreateConfig(firecracker.WithMicroVM(vm))
	g.Expect(err).To(g.MatchError(errors.ErrInvalidImageFilePath))
}
