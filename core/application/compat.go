package application

import (
	"errors"
	"fmt"
	"strings"

	"github.com/liquidmetal-dev/flintlock/core/models"
)

var errCompatibilityInput = errors.New("checking compatibility needs a descriptor and host facts")

// FieldMismatch is one descriptor field whose value differs from the host.
type FieldMismatch struct {
	// Field is the JSON name of the descriptor field.
	Field string
	// Snapshot is the value in the descriptor.
	Snapshot string
	// Host is the value on the host.
	Host string
}

// CompatibilityError tells that a snapshot cannot be restored on this host. It
// names each field that differs, with both values.
type CompatibilityError struct {
	Mismatches []FieldMismatch
}

// Error returns the error message.
func (e *CompatibilityError) Error() string {
	parts := make([]string, 0, len(e.Mismatches))
	for _, m := range e.Mismatches {
		parts = append(parts, fmt.Sprintf("%s: snapshot %q, host %q", m.Field, m.Snapshot, m.Host))
	}

	return "snapshot is not compatible with this host: " + strings.Join(parts, "; ")
}

// CheckCompatibility compares the compatibility descriptor of a snapshot with
// the facts of this host. It returns a *CompatibilityError that names each
// field that differs.
//
// It always enforces vmm_type, arch and cpu_vendor, plus fc_snapshot_version
// on Firecracker or the exact vmm_version on Cloud Hypervisor. It enforces
// host_kernel, cpu_model, cpu_microcode, and vmm_version on Firecracker,
// unless force is set.
func CheckCompatibility(desc *models.SnapshotDescriptor, facts *models.HostFacts, force bool) error {
	if desc == nil || facts == nil {
		return errCompatibilityInput
	}

	var mismatches []FieldMismatch

	check := func(field, snapshot, host string) {
		if snapshot != host {
			mismatches = append(mismatches, FieldMismatch{Field: field, Snapshot: snapshot, Host: host})
		}
	}

	// The checks that force does not skip.
	check("vmm_type", desc.VMMType, facts.VMMType)
	check("arch", desc.Arch, facts.Arch)
	check("cpu_vendor", desc.CPUVendor, facts.CPUVendor)

	// The VMM version checks only mean something when the VMM type is the
	// same.
	sameVMM := desc.VMMType == facts.VMMType

	if sameVMM {
		switch desc.VMMType {
		case models.VMMTypeFirecracker:
			check("fc_snapshot_version", desc.FCSnapshotVersion, facts.FCSnapshotVersion)
		case models.VMMTypeCloudHypervisor:
			check("vmm_version", desc.VMMVersion, facts.VMMVersion)
		}
	}

	// The checks that force skips.
	if !force {
		check("host_kernel", desc.HostKernel, facts.HostKernel)
		check("cpu_model", desc.CPUModel, facts.CPUModel)
		check("cpu_microcode", desc.CPUMicrocode, facts.CPUMicrocode)

		if sameVMM && desc.VMMType == models.VMMTypeFirecracker {
			check("vmm_version", desc.VMMVersion, facts.VMMVersion)
		}
	}

	if len(mismatches) > 0 {
		return &CompatibilityError{Mismatches: mismatches}
	}

	return nil
}
