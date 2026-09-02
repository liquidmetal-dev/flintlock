package firecracker_test

import (
	"strings"
	"testing"

	g "github.com/onsi/gomega"
	"github.com/spf13/afero"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/infrastructure/microvm/firecracker"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
)

func testVSockState(t *testing.T) firecracker.State {
	t.Helper()

	vmid, err := models.NewVMID("test", "ns", "344780b0-6249-11ec-90d6-0242ac120003")
	g.Expect(err).NotTo(g.HaveOccurred())

	return firecracker.NewState(*vmid, "/var/lib/flintlock", afero.NewMemMapFs())
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
func vmForMicroVM(cpuConfig *models.CPUConfig) *models.MicroVM {
	return &models.MicroVM{
		Spec: models.MicroVMSpec{
			VCPU:       1,
			MemoryInMb: 1024,
			Kernel:     models.Kernel{Filename: "vmlinux"},
			RootVolume: models.Volume{ID: "root"},
			CPUConfig:  cpuConfig,
		},
		Status: models.MicroVMStatus{
			KernelMount: &models.Mount{Source: "/kernel"},
			Volumes: models.VolumeStatuses{
				"root": &models.VolumeStatus{Mount: models.Mount{Source: "/root.img"}},
			},
		},
	}
}

func TestWithMicroVM_CPUConfig_EnableAndDisable(t *testing.T) {
	g.RegisterTestingT(t)

	vm := vmForMicroVM(&models.CPUConfig{
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

	vm := vmForMicroVM(nil)

	cfg, err := firecracker.CreateConfig(firecracker.WithMicroVM(vm))
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(cfg.CPUConfig).To(g.BeNil())
}

func TestWithMicroVM_CPUConfig_Empty(t *testing.T) {
	g.RegisterTestingT(t)

	vm := vmForMicroVM(&models.CPUConfig{})

	cfg, err := firecracker.CreateConfig(firecracker.WithMicroVM(vm))
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(cfg.CPUConfig).To(g.BeNil())
}
