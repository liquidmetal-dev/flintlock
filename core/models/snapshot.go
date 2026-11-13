package models

import "time"

// SnapshotPhase is the phase of a snapshot.
type SnapshotPhase string

const (
	SnapshotPhasePending   SnapshotPhase = "pending"
	SnapshotPhaseQuiescing SnapshotPhase = "quiescing"
	SnapshotPhaseCapturing SnapshotPhase = "capturing"
	SnapshotPhasePackaging SnapshotPhase = "packaging"
	SnapshotPhasePushing   SnapshotPhase = "pushing"
	SnapshotPhaseReady     SnapshotPhase = "ready"
	SnapshotPhaseFailed    SnapshotPhase = "failed"
)

// InProgress reports whether a snapshot in the phase is still being taken: every
// phase but ready and failed.
func (p SnapshotPhase) InProgress() bool {
	return p != SnapshotPhaseReady && p != SnapshotPhaseFailed
}

// SnapshotAfter selects what happens to the source microvm once the snapshot is taken.
type SnapshotAfter string

const (
	// SnapshotAfterResume resumes the source microvm.
	SnapshotAfterResume SnapshotAfter = "resume"
	// SnapshotAfterStop stops the source microvm.
	SnapshotAfterStop SnapshotAfter = "stop"
)

// CreateSnapshotRequest is a request to take a snapshot of a microvm.
type CreateSnapshotRequest struct {
	// SourceUID is the uid of the microvm to snapshot.
	SourceUID string
	// Name is the optional name of the snapshot. When empty, the store makes one from the
	// name of the source microvm.
	Name string
	// Labels are the labels of the snapshot.
	Labels map[string]string
	// After selects what happens to the source microvm once the snapshot is taken. An
	// empty value means SnapshotAfterResume.
	After SnapshotAfter
}

// MemoryMode selects how the guest memory of a microvm is restored from a snapshot.
type MemoryMode string

const (
	// MemoryModeDefault uses the default memory mode of the provider.
	MemoryModeDefault MemoryMode = ""
	// MemoryModeFile maps the snapshot memory file into the guest.
	MemoryModeFile MemoryMode = "file"
	// MemoryModeCopy copies the snapshot memory file into the guest memory.
	MemoryModeCopy MemoryMode = "copy"
)

// Snapshot represents a snapshot of a microvm.
type Snapshot struct {
	// UID is the unique identifier of the snapshot.
	UID string `json:"uid"`
	// Name is the optional name of the snapshot. It is unique in its namespace.
	Name string `json:"name,omitempty"`
	// Namespace is the namespace of the snapshot. It is the namespace of the source microvm.
	Namespace string `json:"namespace"`
	// Source is the identifier of the microvm the snapshot was taken from.
	Source VMID `json:"source"`
	// Labels allows you to include extra data for the snapshot.
	Labels map[string]string `json:"labels,omitempty"`
	// Status is the status of the snapshot.
	Status SnapshotStatus `json:"status"`
	// CreatedAt is the time the snapshot was created at.
	CreatedAt time.Time `json:"created_at"`
}

// SnapshotStatus contains the status of a snapshot.
type SnapshotStatus struct {
	// Phase is the phase of the snapshot.
	Phase SnapshotPhase `json:"phase"`
	// Error is the error message when the phase is failed.
	Error string `json:"error,omitempty"`
	// LocalPath is the directory of the snapshot on the host when the phase is ready.
	LocalPath string `json:"local_path,omitempty"`
}

// SnapshotRef references a snapshot by its UID, or by its name and namespace.
type SnapshotRef struct {
	// UID is the unique identifier of the snapshot.
	UID string `json:"uid,omitempty"`
	// Name is the name of the snapshot.
	Name string `json:"name,omitempty"`
	// Namespace is the namespace of the snapshot.
	Namespace string `json:"namespace,omitempty"`
}

// SnapshotSource is the source of a microvm that is restored from a snapshot.
type SnapshotSource struct {
	// Snapshot is the snapshot to restore from.
	Snapshot SnapshotRef `json:"snapshot"`
	// Name is the name of the restored microvm.
	Name string `json:"name"`
	// Namespace is the namespace of the restored microvm. If empty, the namespace of the
	// source microvm is used.
	Namespace string `json:"namespace,omitempty"`
	// UID is the unique identifier of the restored microvm. If empty, one is generated.
	UID string `json:"uid,omitempty"`
	// Labels are the labels of the restored microvm. If empty, the labels of the source
	// microvm are used.
	Labels map[string]string `json:"labels,omitempty"`
	// Metadata is the metadata of the restored microvm. If empty, the metadata of the
	// source microvm is used.
	Metadata map[string]string `json:"metadata,omitempty"`
	// NetworkOverrides replace the host devices of the restored microvm's network interfaces.
	NetworkOverrides []HostNetworkOverride `json:"network_overrides,omitempty"`
	// MemoryMode selects how the guest memory is restored.
	MemoryMode MemoryMode `json:"memory_mode,omitempty"`
	// Force skips the compatibility checks that can be skipped.
	Force bool `json:"force,omitempty"`
}

// HostNetworkOverride replaces the host device of a restored microvm's network interface.
type HostNetworkOverride struct {
	// GuestDeviceName is the name of the network interface in the guest.
	GuestDeviceName string `json:"guest_device_name"`
	// HostDeviceName is the name of the host device to use for the network interface.
	HostDeviceName string `json:"host_device_name"`
}
