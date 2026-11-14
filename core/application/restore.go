package application

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"reflect"
	"strconv"

	"github.com/spf13/afero"

	coreerrs "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
	"github.com/liquidmetal-dev/flintlock/pkg/validation"
)

// restoreMicroVM creates a microvm from a snapshot. The spec is the spec of
// the source with the overrides of the snapshot source. The plan restores it.
func (a *app) restoreMicroVM(ctx context.Context, req *models.MicroVM) (*models.MicroVM, error) {
	source := req.Spec.SnapshotSource

	logger := log.GetLogger(ctx).WithField("component", "app")
	logger.Debugf("restoring microvm %s from snapshot %s", source.Name, snapshotRefString(source.Snapshot))

	if err := checkRestoreRequest(req); err != nil {
		return nil, err
	}

	snapshot, err := a.ports.SnapshotRepo.Get(ctx, source.Snapshot)
	if err != nil {
		return nil, fmt.Errorf("getting snapshot %s: %w", snapshotRefString(source.Snapshot), err)
	}

	if snapshot.Status.Phase != models.SnapshotPhaseReady {
		return nil, coreerrs.NewFailedPrecondition(fmt.Sprintf(
			"snapshot %s is %s: only a ready snapshot can be restored", snapshot.UID, snapshot.Status.Phase,
		))
	}

	dir := a.ports.SnapshotRepo.Dir(snapshot.UID)

	desc, sourceSpec, err := a.readSnapshotFiles(dir)
	if err != nil {
		return nil, err
	}

	if req.Spec.Provider != "" && req.Spec.Provider != sourceSpec.Provider {
		return nil, coreerrs.NewInvalidArgument(fmt.Sprintf(
			"provider %s is not the provider of the snapshot, %s", req.Spec.Provider, sourceSpec.Provider,
		))
	}

	provider, ok := a.ports.MicrovmProviders[sourceSpec.Provider]
	if !ok {
		return nil, coreerrs.NewFailedPrecondition(
			fmt.Sprintf("microvm provider %s of snapshot %s isn't available", sourceSpec.Provider, snapshot.UID),
		)
	}

	if err := a.checkRestoreCompatibility(ctx, desc, sourceSpec.Provider, source.Force); err != nil {
		return nil, err
	}

	mvm, err := a.buildRestoredMicroVM(snapshot, sourceSpec, source)
	if err != nil {
		return nil, err
	}

	if err := checkSnapshotCapability(provider); err != nil {
		return nil, err
	}

	if err := checkProviderCapabilities(mvm, provider); err != nil {
		return nil, err
	}

	mode, err := restoreMemoryMode(sourceSpec.Provider, source.MemoryMode)
	if err != nil {
		return nil, err
	}

	mvm.Spec.SnapshotSource.MemoryMode = mode

	validator := validation.NewValidator()
	if validErr := validator.ValidateStruct(mvm); validErr != nil {
		return nil, fmt.Errorf("an error occurred when attempting to validate the restored microvm spec: %w", validErr)
	}

	if err := a.checkMicroVMNotExists(ctx, mvm.ID); err != nil {
		return nil, err
	}

	generation, err := a.ports.SnapshotRepo.NextGeneration(ctx, snapshot.UID)
	if err != nil {
		return nil, fmt.Errorf("taking the restore generation of snapshot %s: %w", snapshot.UID, err)
	}

	mvm.Spec.SnapshotSource.Generation = generation
	addRestoreMarkers(mvm, snapshot.UID, generation)

	return a.saveNewMicroVM(ctx, mvm)
}

// checkRestoreRequest rejects a restore request that sets a part of the spec. The
// spec of a restored microvm is the spec of the snapshot source, and only the
// snapshot source can override it.
func checkRestoreRequest(req *models.MicroVM) error {
	source := req.Spec.SnapshotSource

	if source.Name == "" {
		return coreerrs.NewInvalidArgument("snapshot_source.name is required")
	}

	if source.Snapshot.UID == "" && (source.Snapshot.Name == "" || source.Snapshot.Namespace == "") {
		return coreerrs.NewInvalidArgument("snapshot_source.snapshot needs a uid, or a name and a namespace")
	}

	if !req.ID.IsEmpty() {
		return errSetByRestore("id")
	}

	spec := req.Spec

	fields := []struct {
		name string
		set  bool
	}{
		{"vcpu", spec.VCPU != 0},
		{"memory_inmb", spec.MemoryInMb != 0},
		{"kernel", !reflect.ValueOf(spec.Kernel).IsZero()},
		{"initrd", spec.Initrd != nil},
		{"root_volume", !reflect.ValueOf(spec.RootVolume).IsZero()},
		{"additional_volumes", len(spec.AdditionalVolumes) > 0},
		{"network_interfaces", len(spec.NetworkInterfaces) > 0},
		{"metadata", len(spec.Metadata) > 0},
		{"allow_guest_agent", spec.AllowGuestAgent},
		{"cpu_config", spec.CPUConfig != nil},
	}

	for _, field := range fields {
		if field.set {
			return errSetByRestore(field.name)
		}
	}

	return nil
}

func errSetByRestore(field string) error {
	return coreerrs.NewInvalidArgument(
		field + " cannot be set for a restore: it comes from the snapshot, and only snapshot_source overrides it",
	)
}

// readSnapshotFiles reads the compatibility descriptor and the source spec in
// the snapshot directory dir.
func (a *app) readSnapshotFiles(dir string) (*models.SnapshotDescriptor, *models.MicroVMSpec, error) {
	desc := &models.SnapshotDescriptor{}
	if err := readJSONFile(a.ports.FileSystem, filepath.Join(dir, models.SnapshotDescriptorFilename), desc); err != nil {
		return nil, nil, err
	}

	spec := &models.MicroVMSpec{}
	if err := readJSONFile(a.ports.FileSystem, filepath.Join(dir, models.SnapshotSpecFilename), spec); err != nil {
		return nil, nil, err
	}

	return desc, spec, nil
}

func readJSONFile(fs afero.Fs, path string, out any) error {
	data, err := afero.ReadFile(fs, path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}

	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decoding %s: %w", path, err)
	}

	return nil
}

// checkRestoreCompatibility compares the descriptor of a snapshot with the
// facts of this host for the provider.
func (a *app) checkRestoreCompatibility(
	ctx context.Context,
	desc *models.SnapshotDescriptor,
	provider string,
	force bool,
) error {
	facts, err := a.ports.HostInfoService.Facts(ctx, provider)
	if err != nil {
		return fmt.Errorf("getting the host facts for provider %s: %w", provider, err)
	}

	err = CheckCompatibility(desc, facts, force)

	var compatErr *CompatibilityError
	if errors.As(err, &compatErr) {
		return coreerrs.NewFailedPrecondition(compatErr.Error())
	}

	return err
}

// buildRestoredMicroVM makes the microvm of a restore from the spec of the
// source and the overrides of the snapshot source. The devices, their order,
// the vcpus, the memory, the kernel and the initrd are always those of the source.
func (a *app) buildRestoredMicroVM(
	snapshot *models.Snapshot,
	sourceSpec *models.MicroVMSpec,
	source *models.SnapshotSource,
) (*models.MicroVM, error) {
	if !sourceSpec.AllowGuestAgent {
		return nil, coreerrs.NewFailedPrecondition(fmt.Sprintf(
			"the source of snapshot %s does not have allow_guest_agent set: a restore waits for the guest agent",
			snapshot.UID,
		))
	}

	namespace := source.Namespace
	if namespace == "" {
		namespace = snapshot.Source.Namespace()
	}

	uid := source.UID
	if uid == "" {
		generated, err := a.ports.IdentifierService.GenerateRandom()
		if err != nil {
			return nil, fmt.Errorf("generating random ID for microvm: %w", err)
		}

		uid = generated
	}

	vmid, err := models.NewVMID(source.Name, namespace, uid)
	if err != nil {
		return nil, fmt.Errorf("creating vmid: %w", err)
	}

	spec := *sourceSpec
	spec.CreatedAt = 0
	spec.UpdatedAt = 0
	spec.DeletedAt = 0

	spec.Metadata = maps.Clone(sourceSpec.Metadata)
	if len(source.Metadata) > 0 {
		spec.Metadata = maps.Clone(source.Metadata)
	}

	if spec.Metadata == nil {
		spec.Metadata = map[string]string{}
	}

	recorded := *source
	recorded.Snapshot = models.SnapshotRef{UID: snapshot.UID}
	recorded.Namespace = namespace
	recorded.UID = uid
	recorded.Restored = false
	spec.SnapshotSource = &recorded

	mvm := &models.MicroVM{
		ID:   *vmid,
		Spec: spec,
	}

	if err := applyNetworkOverrides(mvm, source.NetworkOverrides); err != nil {
		return nil, err
	}

	return mvm, nil
}

// applyNetworkOverrides names the host devices of the interfaces the overrides
// name. The network steps create the other host devices with generated names.
func applyNetworkOverrides(mvm *models.MicroVM, overrides []models.HostNetworkOverride) error {
	mvm.Status.NetworkInterfaces = models.NetworkInterfaceStatuses{}

	for _, override := range overrides {
		if !hasGuestDevice(mvm.Spec.NetworkInterfaces, override.GuestDeviceName) {
			return coreerrs.NewInvalidArgument(fmt.Sprintf(
				"network override for %s: the snapshot has no interface with that guest device name",
				override.GuestDeviceName,
			))
		}

		if _, ok := mvm.Status.NetworkInterfaces[override.GuestDeviceName]; ok {
			return coreerrs.NewInvalidArgument("more than one network override for " + override.GuestDeviceName)
		}

		mvm.Status.NetworkInterfaces[override.GuestDeviceName] = &models.NetworkInterfaceStatus{
			HostDeviceName: override.HostDeviceName,
		}
	}

	return nil
}

func hasGuestDevice(ifaces []models.NetworkInterface, guestDeviceName string) bool {
	for i := range ifaces {
		if ifaces[i].GuestDeviceName == guestDeviceName {
			return true
		}
	}

	return false
}

// restoreMemoryMode returns the memory mode of a restore with the provider. An
// empty mode is the default of the provider.
func restoreMemoryMode(provider string, requested models.MemoryMode) (models.MemoryMode, error) {
	var supported models.MemoryMode

	switch provider {
	case models.VMMTypeFirecracker:
		supported = models.MemoryModeFile
	case models.VMMTypeCloudHypervisor:
		supported = models.MemoryModeCopy
	default:
		return "", coreerrs.NewInvalidArgument(
			fmt.Sprintf("microvm provider %s has no memory mode for a restore", provider),
		)
	}

	if requested != models.MemoryModeDefault && requested != supported {
		return "", coreerrs.NewInvalidArgument(fmt.Sprintf(
			"memory mode %s is not supported by microvm provider %s: it supports %s", requested, provider, supported,
		))
	}

	return supported, nil
}

// addRestoreMarkers adds the keys that tell the guest it was restored, and from
// which snapshot, to the metadata. The values are base64 encoded, as all metadata
// values are.
func addRestoreMarkers(mvm *models.MicroVM, snapshotUID string, generation int) {
	markers := map[string]string{
		models.MetadataSnapshotUID:       snapshotUID,
		models.MetadataRestoreUID:        mvm.ID.UID(),
		models.MetadataRestoreGeneration: strconv.Itoa(generation),
	}

	for key, value := range markers {
		mvm.Spec.Metadata[key] = base64.StdEncoding.EncodeToString([]byte(value))
	}
}
