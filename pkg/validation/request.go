package validation

import (
	"errors"

	mvmv1 "github.com/liquidmetal-dev/flintlock/api/services/microvm/v1alpha1"
	snapv1 "github.com/liquidmetal-dev/flintlock/api/services/snapshot/v1alpha1"
	"github.com/liquidmetal-dev/flintlock/api/types"
)

var (
	ErrCreateRequestRequired       = errors.New("create microvm request is required")
	ErrMicroVMOrSnapshotSource     = errors.New("exactly one of microvm and snapshot_source is required")
	ErrSnapshotSourceNameRequired  = errors.New("snapshot_source.name is required")
	ErrSnapshotSourceRefRequired   = errors.New("snapshot_source.snapshot is required")
	ErrSnapshotRefInvalid          = errors.New("snapshot reference needs a uid, or a name and a namespace")
	ErrSnapshotMemoryModeUnknown   = errors.New("snapshot_source.memory_mode is not a known memory mode")
	ErrSnapshotNetworkOverrideName = errors.New("snapshot_source.network_overrides need a guest and a host device name")
	ErrCreateSnapshotRequired      = errors.New("create snapshot request is required")
	ErrSnapshotSourceUIDRequired   = errors.New("source_uid is required")
	ErrSnapshotNameEmpty           = errors.New("name must not be empty when it is set")
	ErrSnapshotAfterUnknown        = errors.New("after is not a known value")
)

// ValidateCreateMicroVMRequest checks that a create request has either a microvm spec or a
// snapshot source, and that a snapshot source is complete. The microvm spec itself is
// validated after it is converted to the model.
func ValidateCreateMicroVMRequest(req *mvmv1.CreateMicroVMRequest) error {
	if req == nil {
		return ErrCreateRequestRequired
	}

	hasSpec := req.GetMicrovm() != nil
	hasSource := req.GetSnapshotSource() != nil

	if hasSpec == hasSource {
		return ErrMicroVMOrSnapshotSource
	}

	if hasSource {
		return validateSnapshotSource(req.GetSnapshotSource())
	}

	return nil
}

func validateSnapshotSource(source *types.SnapshotSource) error {
	if source.GetName() == "" {
		return ErrSnapshotSourceNameRequired
	}

	if source.GetSnapshot() == nil {
		return ErrSnapshotSourceRefRequired
	}

	if err := ValidateSnapshotRef(source.GetSnapshot()); err != nil {
		return err
	}

	if _, ok := types.MemoryMode_name[int32(source.GetMemoryMode())]; !ok {
		return ErrSnapshotMemoryModeUnknown
	}

	for _, override := range source.GetNetworkOverrides() {
		if override.GetGuestDeviceName() == "" || override.GetHostDeviceName() == "" {
			return ErrSnapshotNetworkOverrideName
		}
	}

	return nil
}

// ValidateSnapshotRef checks that a snapshot reference has a uid, or a name and a namespace.
func ValidateSnapshotRef(ref *types.SnapshotRef) error {
	switch r := ref.GetRef().(type) {
	case *types.SnapshotRef_Uid:
		if r.Uid != "" {
			return nil
		}
	case *types.SnapshotRef_Named:
		if r.Named.GetName() != "" && r.Named.GetNamespace() != "" {
			return nil
		}
	}

	return ErrSnapshotRefInvalid
}

// ValidateCreateSnapshotRequest checks that a create snapshot request names its source
// microvm. The name is optional as the name of a microvm is required: when set, it must
// not be empty. Labels are free form, as the labels of a microvm are.
func ValidateCreateSnapshotRequest(req *snapv1.CreateSnapshotRequest) error {
	if req == nil {
		return ErrCreateSnapshotRequired
	}

	if req.GetSourceUid() == "" {
		return ErrSnapshotSourceUIDRequired
	}

	if req.Name != nil && req.GetName() == "" {
		return ErrSnapshotNameEmpty
	}

	if _, ok := snapv1.After_name[int32(req.GetAfter())]; !ok {
		return ErrSnapshotAfterUnknown
	}

	return nil
}
