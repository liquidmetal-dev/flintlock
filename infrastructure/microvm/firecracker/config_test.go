package firecracker_test

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	g "github.com/onsi/gomega"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v2"

	"github.com/liquidmetal-dev/flintlock/client/cloudinit/network"
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

func TestWithMicroVM_Balloon_FreePageReporting(t *testing.T) {
	g.RegisterTestingT(t)

	vm := vmForMicroVM(t, nil)
	vm.Spec.Balloon = &models.Balloon{FreePageReporting: true}

	cfg, err := firecracker.CreateConfig(firecracker.WithMicroVM(vm))
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(cfg.Balloon).To(g.Equal(&firecracker.BalloonDeviceConfig{
		AmountMib:         0,
		DeflateOnOOM:      true,
		FreePageReporting: true,
	}))

	data, err := json.Marshal(cfg.Balloon)
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(string(data)).To(g.MatchJSON(
		`{"amount_mib":0,"deflate_on_oom":true,"stats_polling_interval_s":0,"free_page_reporting":true}`))
}

func TestWithMicroVM_Balloon_NoFreePageReporting(t *testing.T) {
	g.RegisterTestingT(t)

	vm := vmForMicroVM(t, nil)
	vm.Spec.Balloon = &models.Balloon{}

	cfg, err := firecracker.CreateConfig(firecracker.WithMicroVM(vm))
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(cfg.Balloon).NotTo(g.BeNil())
	g.Expect(cfg.Balloon.DeflateOnOOM).To(g.BeTrue())
	g.Expect(cfg.Balloon.FreePageReporting).To(g.BeFalse())
}

func TestWithMicroVM_Balloon_Nil(t *testing.T) {
	g.RegisterTestingT(t)

	vm := vmForMicroVM(t, nil)

	cfg, err := firecracker.CreateConfig(firecracker.WithMicroVM(vm))
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(cfg.Balloon).To(g.BeNil())
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

func TestWithMicroVM_KernelNotMounted(t *testing.T) {
	g.RegisterTestingT(t)

	vm := vmForMicroVM(t, nil)
	vm.Status.KernelMount = nil

	_, err := firecracker.CreateConfig(firecracker.WithMicroVM(vm))
	g.Expect(err).To(g.MatchError(errors.ErrNoMount))
}

func TestWithMicroVM_InitrdNotMounted(t *testing.T) {
	g.RegisterTestingT(t)

	vm := vmForMicroVM(t, nil)
	vm.Spec.Initrd = &models.Initrd{Filename: "initrd.img"}
	vm.Status.InitrdMount = nil

	_, err := firecracker.CreateConfig(firecracker.WithMicroVM(vm))
	g.Expect(err).To(g.MatchError(errors.ErrNoMount))
}

func TestWithMicroVM_BlockDevicesInGuestOrder(t *testing.T) {
	g.RegisterTestingT(t)

	vm := vmForMicroVM(t, nil)
	vm.Spec.RootVolume.IsReadOnly = true
	vm.Spec.AdditionalVolumes = models.Volumes{{ID: "data"}, {ID: "logs", IsReadOnly: true}}
	vm.Status.Volumes["data"] = &models.VolumeStatus{Mount: models.Mount{Source: "/data.img"}}
	vm.Status.Volumes["logs"] = &models.VolumeStatus{Mount: models.Mount{Source: "/logs.img"}}

	cfg, err := firecracker.CreateConfig(firecracker.WithMicroVM(vm))

	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(cfg.BlockDevices).To(g.Equal([]firecracker.BlockDeviceConfig{
		{
			ID:           "root",
			PathOnHost:   "/root.img",
			IsRootDevice: true,
			IsReadOnly:   true,
			CacheType:    firecracker.CacheTypeUnsafe,
		},
		{ID: "data", PathOnHost: "/data.img", CacheType: firecracker.CacheTypeUnsafe},
		{ID: "logs", PathOnHost: "/logs.img", IsReadOnly: true, CacheType: firecracker.CacheTypeUnsafe},
	}))
}

func TestWithMicroVM_AdditionalVolumeNotMounted(t *testing.T) {
	g.RegisterTestingT(t)

	vm := vmForMicroVM(t, nil)
	vm.Spec.AdditionalVolumes = models.Volumes{{ID: "data"}}

	_, err := firecracker.CreateConfig(firecracker.WithMicroVM(vm))
	g.Expect(err).To(g.MatchError(errors.NewVolumeNotMounted("data")))
}

func TestProviderDrives(t *testing.T) {
	g.RegisterTestingT(t)

	vm := vmForMicroVM(t, nil)
	vm.Spec.AdditionalVolumes = models.Volumes{{ID: "data"}}
	vm.Status.Volumes["data"] = &models.VolumeStatus{Mount: models.Mount{Source: "/data.img"}}

	provider := firecracker.New(&firecracker.Config{}, nil, afero.NewMemMapFs())

	drives, err := provider.Drives(vm)

	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(drives).To(g.HaveLen(2))
	g.Expect(drives[0].GuestDevice).To(g.Equal("vda"))

	device, found := drives.GuestDeviceForVolume("data")
	g.Expect(found).To(g.BeTrue())
	g.Expect(device).To(g.Equal("vdb"))
}

func TestProviderCapabilities(t *testing.T) {
	g.RegisterTestingT(t)

	provider := firecracker.New(&firecracker.Config{}, nil, afero.NewMemMapFs())
	g.Expect(provider.Capabilities()).NotTo(g.ContainElement(models.VolumeFullCapability))
	g.Expect(provider.Capabilities()).NotTo(g.ContainElement(models.VolumeOverlayCapability))

	provider = firecracker.New(&firecracker.Config{
		VolumeCapabilities: models.Capabilities{models.VolumeOverlayCapability},
	}, nil, afero.NewMemMapFs())
	g.Expect(provider.Capabilities()).To(g.ContainElement(models.VolumeOverlayCapability))
	g.Expect(provider.Capabilities()).NotTo(g.ContainElement(models.VolumeFullCapability))
	g.Expect(provider.Capabilities()).To(g.ContainElement(models.MetadataServiceCapability))
}

// withOverlayRoot puts the root volume in overlay mode and records both
// mounts, as the volume step does.
func withOverlayRoot(vm *models.MicroVM) *models.MicroVM {
	vm.Spec.RootVolume.Mode = models.VolumeModeOverlay
	vm.Status.Volumes["root"] = &models.VolumeStatus{
		Mount:     models.Mount{Type: models.MountTypeFile, Source: "/var/lib/flintlock/vm/ns/test/uid/volumes/root.rw"},
		BaseMount: &models.Mount{Type: models.MountTypeFile, Source: "/var/lib/flintlock/bases/abc.img"},
	}

	return vm
}

func bootParams(cfg *firecracker.VmmConfig) []string {
	return strings.Fields(*cfg.BootSource.BootArgs)
}

func TestWithMicroVM_OverlayBlockDevices(t *testing.T) {
	g.RegisterTestingT(t)

	vm := withOverlayRoot(vmForMicroVM(t, nil))
	vm.Spec.AdditionalVolumes = models.Volumes{{ID: "data"}}
	vm.Status.Volumes["data"] = &models.VolumeStatus{Mount: models.Mount{Source: "/data.img"}}

	cfg, err := firecracker.CreateConfig(firecracker.WithMicroVM(vm))
	g.Expect(err).NotTo(g.HaveOccurred())

	got, err := json.MarshalIndent(cfg.BlockDevices, "", "  ")
	g.Expect(err).NotTo(g.HaveOccurred())

	want, err := os.ReadFile("testdata/overlay_drives.json")
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(string(got)).To(g.Equal(strings.TrimSpace(string(want))))
}

func TestWithMicroVM_OverlayKernelParams(t *testing.T) {
	g.RegisterTestingT(t)

	vm := withOverlayRoot(vmForMicroVM(t, nil))

	cfg, err := firecracker.CreateConfig(firecracker.WithMicroVM(vm))
	g.Expect(err).NotTo(g.HaveOccurred())

	params := bootParams(cfg)
	g.Expect(params).To(g.ContainElements(
		"flintlock.root.contract=1",
		"flintlock.root.base=/dev/vda",
		"flintlock.root.rw=/dev/vdb",
	))
	g.Expect(params).NotTo(g.ContainElement(g.HavePrefix("root=")))
}

func TestWithMicroVM_OverlayKernelParamsReplaceSpec(t *testing.T) {
	g.RegisterTestingT(t)

	vm := withOverlayRoot(vmForMicroVM(t, nil))
	vm.Spec.Kernel.CmdLine = map[string]string{
		"flintlock.root.contract": "2",
		"flintlock.root.base":     "/dev/sda",
		"flintlock.root.rw":       "/dev/sdb",
		"init":                    "/bin/sh",
	}

	cfg, err := firecracker.CreateConfig(firecracker.WithMicroVM(vm))
	g.Expect(err).NotTo(g.HaveOccurred())

	params := bootParams(cfg)
	g.Expect(params).To(g.ContainElements(
		"flintlock.root.contract=1",
		"flintlock.root.base=/dev/vda",
		"flintlock.root.rw=/dev/vdb",
		"init=/bin/sh",
	))
	g.Expect(params).NotTo(g.ContainElements("flintlock.root.contract=2", "flintlock.root.base=/dev/sda"))
}

func TestWithMicroVM_FullModeHasNoOverlayKernelParams(t *testing.T) {
	g.RegisterTestingT(t)

	cfg, err := firecracker.CreateConfig(firecracker.WithMicroVM(vmForMicroVM(t, nil)))
	g.Expect(err).NotTo(g.HaveOccurred())

	g.Expect(bootParams(cfg)).NotTo(g.ContainElement(g.HavePrefix("flintlock.root.")))
	g.Expect(cfg.BlockDevices).To(g.HaveLen(1))
	g.Expect(cfg.BlockDevices[0].PathOnHost).To(g.Equal("/root.img"))
	g.Expect(cfg.BlockDevices[0].IsRootDevice).To(g.BeTrue())
}

func TestWithMicroVM_OverlayBaseNotMounted(t *testing.T) {
	g.RegisterTestingT(t)

	vm := withOverlayRoot(vmForMicroVM(t, nil))
	vm.Status.Volumes["root"].BaseMount = nil

	_, err := firecracker.CreateConfig(firecracker.WithMicroVM(vm))
	g.Expect(err).To(g.MatchError(errors.NewVolumeNotMounted("root")))
}

func TestProviderDrives_Overlay(t *testing.T) {
	g.RegisterTestingT(t)

	vm := withOverlayRoot(vmForMicroVM(t, nil))
	vm.Spec.AdditionalVolumes = models.Volumes{{ID: "data"}}
	vm.Status.Volumes["data"] = &models.VolumeStatus{Mount: models.Mount{Source: "/data.img"}}

	provider := firecracker.New(&firecracker.Config{}, nil, afero.NewMemMapFs())

	drives, err := provider.Drives(vm)

	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(drives).To(g.HaveLen(3))
	g.Expect(drives[0].Role).To(g.Equal(models.DriveRoleRootBase))
	g.Expect(drives[0].Path).To(g.Equal("/var/lib/flintlock/bases/abc.img"))
	g.Expect(drives[1].Role).To(g.Equal(models.DriveRoleRootRW))
	g.Expect(drives[1].GuestDevice).To(g.Equal("vdb"))

	device, found := drives.GuestDeviceForVolume("data")
	g.Expect(found).To(g.BeTrue())
	g.Expect(device).To(g.Equal("vdc"))
}

// An empty address gives eth0 no static address.
func vmWithMetadataInterface(t *testing.T, address string) *models.MicroVM {
	t.Helper()

	vm := vmForMicroVM(t, nil)
	vm.Spec.Kernel.AddNetworkConfig = true

	metadataIface := models.NetworkInterface{
		GuestDeviceName:       "eth0",
		Type:                  models.IfaceTypeTap,
		AllowMetadataRequests: true,
		GuestMAC:              "AA:FF:00:00:00:01",
	}
	if address != "" {
		metadataIface.StaticAddress = &models.StaticAddress{Address: models.IPAddressCIDR(address)}
	}

	vm.Spec.NetworkInterfaces = []models.NetworkInterface{
		metadataIface,
		{
			GuestDeviceName: "eth1",
			Type:            models.IfaceTypeTap,
			GuestMAC:        "AA:FF:00:00:00:02",
			StaticAddress:   &models.StaticAddress{Address: "10.0.0.2/24"},
		},
	}
	vm.Status.NetworkInterfaces = models.NetworkInterfaceStatuses{
		"eth0": &models.NetworkInterfaceStatus{HostDeviceName: "tap0"},
		"eth1": &models.NetworkInterfaceStatus{HostDeviceName: "tap1"},
	}

	return vm
}

func paramsWithPrefix(params []string, prefix string) []string {
	matches := []string{}

	for _, param := range params {
		if strings.HasPrefix(param, prefix) {
			matches = append(matches, param)
		}
	}

	return matches
}

func netplanFromBootParams(t *testing.T, cfg *firecracker.VmmConfig) network.Network {
	t.Helper()

	encoded := paramsWithPrefix(bootParams(cfg), "network-config=")
	g.Expect(encoded).To(g.HaveLen(1))

	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(encoded[0], "network-config="))
	g.Expect(err).NotTo(g.HaveOccurred())

	var netConf network.Network
	g.Expect(yaml.Unmarshal(raw, &netConf)).To(g.Succeed())

	return netConf
}

func TestWithMicroVM_MetadataKernelIP(t *testing.T) {
	g.RegisterTestingT(t)

	vm := vmWithMetadataInterface(t, "169.254.0.1/16")

	cfg, err := firecracker.CreateConfig(firecracker.WithMicroVM(vm, firecracker.WithMetadataKernelIP()))
	g.Expect(err).NotTo(g.HaveOccurred())

	g.Expect(paramsWithPrefix(bootParams(cfg), "ip=")).To(g.Equal([]string{"ip=169.254.0.1:::255.255.0.0:localhost:eth0:off"}))

	netConf := netplanFromBootParams(t, cfg)
	g.Expect(netConf.Ethernet).To(g.HaveKey("eth1"))
	g.Expect(netConf.Ethernet).NotTo(g.HaveKey("eth0"))
}

func TestWithMicroVM_MetadataKernelIPNetmask(t *testing.T) {
	g.RegisterTestingT(t)

	vm := vmWithMetadataInterface(t, "169.254.0.1/24")

	cfg, err := firecracker.CreateConfig(firecracker.WithMicroVM(vm, firecracker.WithMetadataKernelIP()))
	g.Expect(err).NotTo(g.HaveOccurred())

	g.Expect(paramsWithPrefix(bootParams(cfg), "ip=")).To(g.Equal([]string{"ip=169.254.0.1:::255.255.255.0:localhost:eth0:off"}))
}

func TestWithMicroVM_MetadataKernelIPOff(t *testing.T) {
	g.RegisterTestingT(t)

	vm := vmWithMetadataInterface(t, "169.254.0.1/16")

	cfg, err := firecracker.CreateConfig(firecracker.WithMicroVM(vm))
	g.Expect(err).NotTo(g.HaveOccurred())

	g.Expect(paramsWithPrefix(bootParams(cfg), "ip=")).To(g.BeEmpty())

	netConf := netplanFromBootParams(t, cfg)
	g.Expect(netConf.Ethernet).To(g.HaveKey("eth0"))
	g.Expect(netConf.Ethernet["eth0"].Addresses).To(g.Equal([]string{"169.254.0.1/16"}))
}

func TestWithMicroVM_MetadataKernelIPSpecWins(t *testing.T) {
	g.RegisterTestingT(t)

	vm := vmWithMetadataInterface(t, "169.254.0.1/16")
	vm.Spec.Kernel.CmdLine = map[string]string{"ip": "dhcp"}

	cfg, err := firecracker.CreateConfig(firecracker.WithMicroVM(vm, firecracker.WithMetadataKernelIP()))
	g.Expect(err).NotTo(g.HaveOccurred())

	g.Expect(paramsWithPrefix(bootParams(cfg), "ip=")).To(g.Equal([]string{"ip=dhcp"}))
	g.Expect(netplanFromBootParams(t, cfg).Ethernet).To(g.HaveKey("eth0"))
}

func TestWithMicroVM_MetadataKernelIPNoNetworkConfig(t *testing.T) {
	g.RegisterTestingT(t)

	vm := vmWithMetadataInterface(t, "169.254.0.1/16")
	vm.Spec.Kernel.AddNetworkConfig = false

	cfg, err := firecracker.CreateConfig(firecracker.WithMicroVM(vm, firecracker.WithMetadataKernelIP()))
	g.Expect(err).NotTo(g.HaveOccurred())

	g.Expect(paramsWithPrefix(bootParams(cfg), "ip=")).To(g.BeEmpty())
}

func TestWithMicroVM_MetadataKernelIPNoStaticIPv4(t *testing.T) {
	for _, address := range []string{"", "fd00::1/64"} {
		t.Run("address="+address, func(t *testing.T) {
			g.RegisterTestingT(t)

			vm := vmWithMetadataInterface(t, address)

			cfg, err := firecracker.CreateConfig(firecracker.WithMicroVM(vm, firecracker.WithMetadataKernelIP()))
			g.Expect(err).NotTo(g.HaveOccurred())

			g.Expect(paramsWithPrefix(bootParams(cfg), "ip=")).To(g.BeEmpty())
			g.Expect(netplanFromBootParams(t, cfg).Ethernet).To(g.HaveKey("eth0"))
		})
	}
}
