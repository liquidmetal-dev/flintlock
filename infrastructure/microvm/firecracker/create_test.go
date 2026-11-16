package firecracker

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func encodedDefaultVendorData() string {
	return base64.StdEncoding.EncodeToString([]byte(defaultVendorData))
}

func TestMMDSMetadata_AddsDefaultVendorData(t *testing.T) {
	g.RegisterTestingT(t)

	specMetadata := map[string]string{"meta-data": "bWQ=", "user-data": "dWQ="}

	result := mmdsMetadata(specMetadata)

	g.Expect(result).To(g.Equal(map[string]string{
		"meta-data":   "bWQ=",
		"user-data":   "dWQ=",
		"vendor-data": encodedDefaultVendorData(),
	}))
	g.Expect(specMetadata).To(g.HaveLen(2))
	g.Expect(specMetadata).NotTo(g.HaveKey("vendor-data"))
}

func TestMMDSMetadata_KeepsCallerVendorData(t *testing.T) {
	g.RegisterTestingT(t)

	result := mmdsMetadata(map[string]string{"vendor-data": "Y3VzdG9t"})

	g.Expect(result).To(g.HaveKeyWithValue("vendor-data", "Y3VzdG9t"))
}

func TestMMDSMetadata_KeepsEmptyVendorData(t *testing.T) {
	g.RegisterTestingT(t)

	result := mmdsMetadata(map[string]string{"vendor-data": ""})

	g.Expect(result).To(g.HaveKeyWithValue("vendor-data", ""))
}

func TestMMDSMetadata_NilSpecMetadata(t *testing.T) {
	g.RegisterTestingT(t)

	result := mmdsMetadata(nil)

	g.Expect(result).To(g.Equal(map[string]string{"vendor-data": encodedDefaultVendorData()}))
}

func TestWriteMetadata_MMDSFileHasDefaultVendorData(t *testing.T) {
	g.RegisterTestingT(t)

	vmid, err := models.NewVMID("test", "ns", "344780b0-6249-11ec-90d6-0242ac120003")
	g.Expect(err).NotTo(g.HaveOccurred())

	fs := afero.NewMemMapFs()
	vmState := NewState(*vmid, "/var/lib/flintlock/vm", "/run/flintlock", fs)
	g.Expect(fs.MkdirAll(vmState.Root(), 0o755)).To(g.Succeed())

	vm := &models.MicroVM{ID: *vmid}
	vm.Spec.Metadata = map[string]string{
		"meta-data": base64.StdEncoding.EncodeToString([]byte("md")),
		"user-data": base64.StdEncoding.EncodeToString([]byte("ud")),
	}

	g.Expect(writeMetadata(vm, vmState)).To(g.Succeed())

	// fsState.Metadata reads the config file, so the MMDS file is read directly.
	data, err := afero.ReadFile(fs, vmState.MetadataPath())
	g.Expect(err).NotTo(g.HaveOccurred())

	written := Metadata{}
	g.Expect(json.Unmarshal(data, &written)).To(g.Succeed())
	g.Expect(written.Latest).To(g.HaveKeyWithValue("vendor-data", defaultVendorData))
	g.Expect(written.Latest).To(g.HaveKeyWithValue("user-data", "ud"))
	g.Expect(vm.Spec.Metadata).To(g.HaveLen(2))
}

func TestVmmConfig_MetadataKernelIP(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		t.Run(fmt.Sprintf("enabled=%t", enabled), func(t *testing.T) {
			g.RegisterTestingT(t)

			vmid, err := models.NewVMID("test", "ns", "344780b0-6249-11ec-90d6-0242ac120003")
			g.Expect(err).NotTo(g.HaveOccurred())

			kernelDir := t.TempDir()
			g.Expect(os.WriteFile(filepath.Join(kernelDir, "vmlinux"), []byte("kernel"), 0o600)).To(g.Succeed())

			vm := &models.MicroVM{
				ID: *vmid,
				Spec: models.MicroVMSpec{
					VCPU:       1,
					MemoryInMb: 1024,
					Kernel:     models.Kernel{Filename: "vmlinux", AddNetworkConfig: true},
					RootVolume: models.Volume{ID: "root"},
					NetworkInterfaces: []models.NetworkInterface{{
						GuestDeviceName:       "eth0",
						Type:                  models.IfaceTypeTap,
						AllowMetadataRequests: true,
						GuestMAC:              "AA:FF:00:00:00:01",
						StaticAddress:         &models.StaticAddress{Address: "169.254.0.1/16"},
					}},
				},
				Status: models.MicroVMStatus{
					KernelMount: &models.Mount{Source: kernelDir},
					Volumes: models.VolumeStatuses{
						"root": &models.VolumeStatus{Mount: models.Mount{Source: "/root.img"}},
					},
					NetworkInterfaces: models.NetworkInterfaceStatuses{
						"eth0": &models.NetworkInterfaceStatus{HostDeviceName: "tap0"},
					},
				},
			}

			p := &fcProvider{config: &Config{MetadataKernelIP: enabled}, fs: afero.NewMemMapFs()}
			vmState := NewState(*vmid, "/var/lib/flintlock/vm", "/run/flintlock", p.fs)

			cfg, err := p.vmmConfig(vm, vmState)
			g.Expect(err).NotTo(g.HaveOccurred())

			params := strings.Fields(*cfg.BootSource.BootArgs)
			if enabled {
				g.Expect(params).To(g.ContainElement("ip=169.254.0.1:::255.255.0.0:localhost:eth0:off"))
			} else {
				g.Expect(params).NotTo(g.ContainElement(g.HavePrefix("ip=")))
			}
		})
	}
}
