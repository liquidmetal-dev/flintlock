package application_test

import (
	"errors"
	"testing"

	g "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/core/application"
	"github.com/liquidmetal-dev/flintlock/core/models"
)

func firecrackerFacts() *models.HostFacts {
	return &models.HostFacts{
		VMMType:           models.VMMTypeFirecracker,
		VMMVersion:        "v1.16.1",
		FCSnapshotVersion: "v4.0.0",
		Arch:              "amd64",
		CPUVendor:         "GenuineIntel",
		CPUModel:          "Intel(R) Xeon(R) Platinum 8488C",
		CPUMicrocode:      "0x2b000603",
		HostKernel:        "6.8.0-45-generic",
	}
}

func cloudHypervisorFacts() *models.HostFacts {
	facts := firecrackerFacts()
	facts.VMMType = models.VMMTypeCloudHypervisor
	facts.VMMVersion = "v41.0.0"
	facts.FCSnapshotVersion = ""

	return facts
}

// descriptorFor returns a descriptor taken on a host with the given facts.
func descriptorFor(facts *models.HostFacts) *models.SnapshotDescriptor {
	return &models.SnapshotDescriptor{
		FormatVersion:     models.SnapshotDescriptorFormatVersion,
		VMMType:           facts.VMMType,
		VMMVersion:        facts.VMMVersion,
		FCSnapshotVersion: facts.FCSnapshotVersion,
		Arch:              facts.Arch,
		CPUVendor:         facts.CPUVendor,
		CPUModel:          facts.CPUModel,
		CPUMicrocode:      facts.CPUMicrocode,
		HostKernel:        facts.HostKernel,
	}
}

func TestCheckCompatibility_EqualFacts(t *testing.T) {
	g.RegisterTestingT(t)

	for _, facts := range []*models.HostFacts{firecrackerFacts(), cloudHypervisorFacts()} {
		g.Expect(application.CheckCompatibility(descriptorFor(facts), facts, false)).To(g.Succeed())
		g.Expect(application.CheckCompatibility(descriptorFor(facts), facts, true)).To(g.Succeed())
	}
}

func TestCheckCompatibility_NilInput(t *testing.T) {
	g.RegisterTestingT(t)

	facts := firecrackerFacts()
	g.Expect(application.CheckCompatibility(nil, facts, false)).NotTo(g.Succeed())
	g.Expect(application.CheckCompatibility(descriptorFor(facts), nil, false)).NotTo(g.Succeed())
}

func TestCheckCompatibility_Fields(t *testing.T) {
	testCases := []struct {
		name     string
		facts    func() *models.HostFacts
		change   func(h *models.HostFacts)
		field    string
		snapshot string
		host     string
		// enforced is true when force does not skip the check.
		enforced bool
	}{
		{
			name:  "firecracker arch",
			facts: firecrackerFacts, change: func(h *models.HostFacts) { h.Arch = "arm64" },
			field: "arch", snapshot: "amd64", host: "arm64", enforced: true,
		},
		{
			name:  "firecracker cpu_vendor",
			facts: firecrackerFacts, change: func(h *models.HostFacts) { h.CPUVendor = "AuthenticAMD" },
			field: "cpu_vendor", snapshot: "GenuineIntel", host: "AuthenticAMD", enforced: true,
		},
		{
			name:  "firecracker fc_snapshot_version",
			facts: firecrackerFacts, change: func(h *models.HostFacts) { h.FCSnapshotVersion = "v5.0.0" },
			field: "fc_snapshot_version", snapshot: "v4.0.0", host: "v5.0.0", enforced: true,
		},
		{
			name:  "firecracker vmm_version",
			facts: firecrackerFacts, change: func(h *models.HostFacts) { h.VMMVersion = "v1.17.0" },
			field: "vmm_version", snapshot: "v1.16.1", host: "v1.17.0", enforced: false,
		},
		{
			name:  "firecracker host_kernel",
			facts: firecrackerFacts, change: func(h *models.HostFacts) { h.HostKernel = "6.11.0-1-generic" },
			field: "host_kernel", snapshot: "6.8.0-45-generic", host: "6.11.0-1-generic", enforced: false,
		},
		{
			name:  "firecracker cpu_model",
			facts: firecrackerFacts, change: func(h *models.HostFacts) { h.CPUModel = "Intel(R) Xeon(R) Gold 6338" },
			field: "cpu_model", snapshot: "Intel(R) Xeon(R) Platinum 8488C", host: "Intel(R) Xeon(R) Gold 6338",
			enforced: false,
		},
		{
			name:  "firecracker cpu_microcode",
			facts: firecrackerFacts, change: func(h *models.HostFacts) { h.CPUMicrocode = "0x2b000620" },
			field: "cpu_microcode", snapshot: "0x2b000603", host: "0x2b000620", enforced: false,
		},
		{
			name:  "cloud hypervisor vmm_version",
			facts: cloudHypervisorFacts, change: func(h *models.HostFacts) { h.VMMVersion = "v42.0.0" },
			field: "vmm_version", snapshot: "v41.0.0", host: "v42.0.0", enforced: true,
		},
		{
			name:  "cloud hypervisor host_kernel",
			facts: cloudHypervisorFacts, change: func(h *models.HostFacts) { h.HostKernel = "6.11.0-1-generic" },
			field: "host_kernel", snapshot: "6.8.0-45-generic", host: "6.11.0-1-generic", enforced: false,
		},
		{
			name:  "vmm_type",
			facts: firecrackerFacts, change: func(h *models.HostFacts) { h.VMMType = models.VMMTypeCloudHypervisor },
			field: "vmm_type", snapshot: models.VMMTypeFirecracker, host: models.VMMTypeCloudHypervisor,
			enforced: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g.RegisterTestingT(t)

			desc := descriptorFor(tc.facts())
			host := tc.facts()
			tc.change(host)

			err := application.CheckCompatibility(desc, host, false)
			g.Expect(err).To(g.HaveOccurred())

			var compatErr *application.CompatibilityError
			g.Expect(errors.As(err, &compatErr)).To(g.BeTrue())
			g.Expect(compatErr.Mismatches).To(g.Equal([]application.FieldMismatch{
				{Field: tc.field, Snapshot: tc.snapshot, Host: tc.host},
			}))

			err = application.CheckCompatibility(desc, host, true)
			if tc.enforced {
				g.Expect(errors.As(err, &compatErr)).To(g.BeTrue())
				g.Expect(compatErr.Mismatches[0].Field).To(g.Equal(tc.field))
			} else {
				g.Expect(err).NotTo(g.HaveOccurred())
			}
		})
	}
}

func TestCheckCompatibility_ForceSkipsOnlyShouldFields(t *testing.T) {
	g.RegisterTestingT(t)

	desc := descriptorFor(firecrackerFacts())
	host := &models.HostFacts{
		VMMType:           models.VMMTypeFirecracker,
		VMMVersion:        "v1.17.0",
		FCSnapshotVersion: "v5.0.0",
		Arch:              "arm64",
		CPUVendor:         "AuthenticAMD",
		CPUModel:          "AMD EPYC 7R13 Processor",
		CPUMicrocode:      "0xa001173",
		HostKernel:        "6.11.0-1-generic",
	}

	var compatErr *application.CompatibilityError

	err := application.CheckCompatibility(desc, host, false)
	g.Expect(errors.As(err, &compatErr)).To(g.BeTrue())
	g.Expect(fields(compatErr)).To(g.Equal([]string{
		"arch", "cpu_vendor", "fc_snapshot_version", "host_kernel", "cpu_model", "cpu_microcode", "vmm_version",
	}))

	err = application.CheckCompatibility(desc, host, true)
	g.Expect(errors.As(err, &compatErr)).To(g.BeTrue())
	g.Expect(fields(compatErr)).To(g.Equal([]string{"arch", "cpu_vendor", "fc_snapshot_version"}))
}

func TestCompatibilityError_Message(t *testing.T) {
	g.RegisterTestingT(t)

	desc := descriptorFor(firecrackerFacts())
	host := firecrackerFacts()
	host.VMMVersion = "v1.17.0"

	err := application.CheckCompatibility(desc, host, false)
	g.Expect(err).To(g.MatchError(
		`snapshot is not compatible with this host: vmm_version: snapshot "v1.16.1", host "v1.17.0"`,
	))

	host.HostKernel = "6.11.0-1-generic"
	err = application.CheckCompatibility(desc, host, false)
	g.Expect(err).To(g.MatchError(`snapshot is not compatible with this host: ` +
		`host_kernel: snapshot "6.8.0-45-generic", host "6.11.0-1-generic"; ` +
		`vmm_version: snapshot "v1.16.1", host "v1.17.0"`))
}

func fields(err *application.CompatibilityError) []string {
	names := make([]string, 0, len(err.Mismatches))
	for _, m := range err.Mismatches {
		names = append(names, m.Field)
	}

	return names
}
