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

// optionValues returns the values that follow an option, up to the next
// option.
func optionValues(args []string, option string) []string {
	values := []string{}
	collecting := false

	for _, arg := range args {
		switch {
		case arg == option:
			collecting = true
		case strings.HasPrefix(arg, "--"):
			collecting = false
		case collecting:
			values = append(values, arg)
		}
	}

	return values
}

func TestBuildArgs_DisksInGuestOrder(t *testing.T) {
	g.RegisterTestingT(t)

	p, _, state := newTestProvider(t)

	vm := vmForArgs(t, false)
	vm.Spec.AdditionalVolumes = models.Volumes{{ID: "data"}, {ID: "logs"}}
	vm.Status.Volumes["data"] = &models.VolumeStatus{Mount: models.Mount{Source: "/data.img"}}
	vm.Status.Volumes["logs"] = &models.VolumeStatus{Mount: models.Mount{Source: "/logs.img"}}

	args, err := p.buildArgs(vm, state, nil)

	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(optionValues(args, "--disk")).To(g.Equal([]string{
		"path=/root.img",
		fmt.Sprintf("path=%s,readonly=on", state.CloudInitImage()),
		"path=/data.img",
		"path=/logs.img",
	}))
}

func TestBuildArgs_DisksStayTogether(t *testing.T) {
	g.RegisterTestingT(t)

	p, _, state := newTestProvider(t)

	vm := vmForArgs(t, false)
	vm.Spec.AdditionalVolumes = models.Volumes{
		{ID: "shared", Source: models.VolumeSource{VirtioFS: &models.VirtioFSVolumeSource{Path: "/shared"}}},
		{ID: "data"},
	}
	vm.Status.Volumes["shared"] = &models.VolumeStatus{}
	vm.Status.Volumes["data"] = &models.VolumeStatus{Mount: models.Mount{Source: "/data.img"}}

	args, err := p.buildArgs(vm, state, nil)

	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(optionValues(args, "--disk")).To(g.Equal([]string{
		"path=/root.img",
		fmt.Sprintf("path=%s,readonly=on", state.CloudInitImage()),
		"path=/data.img",
	}))
	g.Expect(optionValues(args, "--fs")).To(g.HaveLen(1))
	g.Expect(strings.Join(args, " ")).To(g.ContainSubstring("shared=on"))
}

func TestBuildArgs_AdditionalVolumeNotMounted(t *testing.T) {
	g.RegisterTestingT(t)

	p, _, state := newTestProvider(t)

	vm := vmForArgs(t, false)
	vm.Spec.AdditionalVolumes = models.Volumes{{ID: "data"}}

	_, err := p.buildArgs(vm, state, nil)
	g.Expect(err).To(g.MatchError(cerrors.NewVolumeNotMounted("data")))
}

// withInitrd gives the microvm an initrd whose image is mounted.
func withInitrd(t *testing.T, vm *models.MicroVM) string {
	t.Helper()

	initrdDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(initrdDir, "initrd.img"), []byte("initrd"), 0o600); err != nil {
		t.Fatal(err)
	}

	vm.Spec.Initrd = &models.Initrd{Filename: "initrd.img"}
	vm.Status.InitrdMount = &models.Mount{Source: initrdDir}

	return filepath.Join(initrdDir, "initrd.img")
}

func TestBuildArgs_Initrd(t *testing.T) {
	g.RegisterTestingT(t)

	p, _, state := newTestProvider(t)

	vm := vmForArgs(t, false)
	initrdPath := withInitrd(t, vm)

	args, err := p.buildArgs(vm, state, nil)

	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(optionValues(args, "--initramfs")).To(g.Equal([]string{initrdPath}))
}

func TestBuildArgs_NoInitrd(t *testing.T) {
	g.RegisterTestingT(t)

	p, _, state := newTestProvider(t)

	args, err := p.buildArgs(vmForArgs(t, false), state, nil)

	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(args).NotTo(g.ContainElement("--initramfs"))
}

func TestBuildArgs_InitrdPathTraversalRejected(t *testing.T) {
	g.RegisterTestingT(t)

	p, _, state := newTestProvider(t)

	vm := vmForArgs(t, false)
	withInitrd(t, vm)
	vm.Spec.Initrd.Filename = "../../../../../../etc/passwd"

	_, err := p.buildArgs(vm, state, nil)

	g.Expect(err).To(g.MatchError(cerrors.ErrInvalidImageFilePath))
}

func TestBuildArgs_InitrdNotMounted(t *testing.T) {
	g.RegisterTestingT(t)

	p, _, state := newTestProvider(t)

	vm := vmForArgs(t, false)
	vm.Spec.Initrd = &models.Initrd{Filename: "initrd.img"}

	_, err := p.buildArgs(vm, state, nil)

	g.Expect(err).To(g.MatchError(cerrors.ErrNoMount))
}

// cmdLineParams returns the parameters of the kernel command line.
func cmdLineParams(t *testing.T, args []string) []string {
	t.Helper()

	values := optionValues(args, "--cmdline")
	g.Expect(values).To(g.HaveLen(1))

	return strings.Fields(values[0])
}

func TestBuildArgs_ReadOnlyVolumes(t *testing.T) {
	g.RegisterTestingT(t)

	p, _, state := newTestProvider(t)

	vm := vmForArgs(t, false)
	vm.Spec.RootVolume.IsReadOnly = true
	vm.Spec.AdditionalVolumes = models.Volumes{{ID: "data"}, {ID: "logs", IsReadOnly: true}}
	vm.Status.Volumes["data"] = &models.VolumeStatus{Mount: models.Mount{Source: "/data.img"}}
	vm.Status.Volumes["logs"] = &models.VolumeStatus{Mount: models.Mount{Source: "/logs.img"}}

	args, err := p.buildArgs(vm, state, nil)

	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(optionValues(args, "--disk")).To(g.Equal([]string{
		"path=/root.img,readonly=on",
		fmt.Sprintf("path=%s,readonly=on", state.CloudInitImage()),
		"path=/data.img",
		"path=/logs.img,readonly=on",
	}))
}

func TestBuildArgs_WritableRootIsMountedReadWrite(t *testing.T) {
	g.RegisterTestingT(t)

	p, _, state := newTestProvider(t)

	args, err := p.buildArgs(vmForArgs(t, false), state, nil)

	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(cmdLineParams(t, args)).To(g.ContainElement("rw"))
	g.Expect(cmdLineParams(t, args)).NotTo(g.ContainElement("ro"))
}

func TestBuildArgs_ReadOnlyRootIsMountedReadOnly(t *testing.T) {
	g.RegisterTestingT(t)

	p, _, state := newTestProvider(t)

	vm := vmForArgs(t, false)
	vm.Spec.RootVolume.IsReadOnly = true

	args, err := p.buildArgs(vm, state, nil)

	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(cmdLineParams(t, args)).To(g.ContainElement("ro"))
	g.Expect(cmdLineParams(t, args)).NotTo(g.ContainElement("rw"))
	g.Expect(cmdLineParams(t, args)).To(g.ContainElement("root=/dev/vda"))
}

func TestBuildArgs_ReadOnlyRootSpecWins(t *testing.T) {
	g.RegisterTestingT(t)

	p, _, state := newTestProvider(t)

	vm := vmForArgs(t, false)
	vm.Spec.RootVolume.IsReadOnly = true
	vm.Spec.Kernel.CmdLine = map[string]string{"rw": ""}

	args, err := p.buildArgs(vm, state, nil)

	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(cmdLineParams(t, args)).To(g.ContainElement("rw"))
	g.Expect(cmdLineParams(t, args)).NotTo(g.ContainElement("ro"))
}

func TestBuildArgs_WritableRootSpecWins(t *testing.T) {
	g.RegisterTestingT(t)

	p, _, state := newTestProvider(t)

	vm := vmForArgs(t, false)
	vm.Spec.Kernel.CmdLine = map[string]string{"ro": ""}

	args, err := p.buildArgs(vm, state, nil)

	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(cmdLineParams(t, args)).To(g.ContainElement("ro"))
	g.Expect(cmdLineParams(t, args)).NotTo(g.ContainElement("rw"))
}

func TestProviderDrives(t *testing.T) {
	g.RegisterTestingT(t)

	p, _, state := newTestProvider(t)

	vm := vmForArgs(t, false)
	vmid, err := models.NewVMID(testVMName, testVMNamespace, testVMUID)
	g.Expect(err).NotTo(g.HaveOccurred())
	vm.ID = *vmid
	vm.Spec.AdditionalVolumes = models.Volumes{{ID: "data"}}
	vm.Status.Volumes["data"] = &models.VolumeStatus{Mount: models.Mount{Source: "/data.img"}}

	drives, err := p.Drives(vm)

	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(drives).To(g.HaveLen(3))
	g.Expect(drives[1].Role).To(g.Equal(models.DriveRoleCloudInit))
	g.Expect(drives[1].Path).To(g.Equal(state.CloudInitImage()))

	device, found := drives.GuestDeviceForVolume("data")
	g.Expect(found).To(g.BeTrue())
	g.Expect(device).To(g.Equal("vdc"))
}
