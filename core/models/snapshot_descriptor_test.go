package models_test

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	g "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/core/models"
)

func TestSnapshotDescriptor_HasNoMetadataField(t *testing.T) {
	g.RegisterTestingT(t)

	descType := reflect.TypeOf(models.SnapshotDescriptor{})
	for i := range descType.NumField() {
		field := descType.Field(i)
		g.Expect(field.Name).NotTo(g.ContainSubstring("Metadata"))
		g.Expect(field.Tag.Get("json")).NotTo(g.ContainSubstring("metadata"))
	}
}

func TestSnapshotDescriptor_MarshalsWithoutSourceMetadata(t *testing.T) {
	g.RegisterTestingT(t)

	secret := "I2Nsb3VkLWNvbmZpZwpwYXNzd29yZDogaHVudGVyMgo="
	spec := models.MicroVMSpec{
		VCPU:       2,
		MemoryInMb: 2048,
		Metadata:   map[string]string{"user-data": secret},
	}

	desc := models.SnapshotDescriptor{
		FormatVersion:     models.SnapshotDescriptorFormatVersion,
		VMMType:           models.VMMTypeFirecracker,
		VMMVersion:        "v1.16.1",
		FCSnapshotVersion: "v4.0.0",
		FlintlockVersion:  "v0.16.0",
		Arch:              "amd64",
		CPUVendor:         "GenuineIntel",
		CPUModel:          "Intel(R) Xeon(R) Platinum 8488C",
		CPUMicrocode:      "0x2b000603",
		HostKernel:        "6.8.0-45-generic",
		VCPUs:             spec.VCPU,
		MemoryMB:          spec.MemoryInMb,
		KernelImage:       models.SnapshotImage{Reference: "ghcr.io/example/kernel:6.1", Digest: "sha256:aaaa"},
		Volumes: []models.SnapshotVolume{
			{DriveID: "root", Order: 0, SizeBytes: 1 << 30, BlockSize: 4096, Filesystem: "ext4", BaseDigest: "sha256:bbbb"},
		},
		CreatedAt: time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC),
		Source:    models.SnapshotSource{UID: "01J", Name: "vm1", Namespace: "ns1"},
		Labels:    map[string]string{"team": "a"},
	}

	data, err := json.Marshal(desc)
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(string(data)).NotTo(g.ContainSubstring("metadata"))
	g.Expect(string(data)).NotTo(g.ContainSubstring(secret))

	var fields map[string]any
	g.Expect(json.Unmarshal(data, &fields)).To(g.Succeed())
	g.Expect(fields).To(g.HaveKeyWithValue("format_version", float64(1)))
	g.Expect(fields).To(g.HaveKeyWithValue("vmm_type", "firecracker"))
	g.Expect(fields).To(g.HaveKeyWithValue("fc_snapshot_version", "v4.0.0"))
	g.Expect(fields).To(g.HaveKeyWithValue("created_at", "2026-11-13T12:00:00Z"))
	g.Expect(fields).NotTo(g.HaveKey("initrd_image"))
}
