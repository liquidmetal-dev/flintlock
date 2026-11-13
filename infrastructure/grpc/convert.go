package grpc

import (
	"fmt"
	"maps"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/liquidmetal-dev/flintlock/api/types"
	"github.com/liquidmetal-dev/flintlock/client/cloudinit/instance"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
	"github.com/liquidmetal-dev/flintlock/pkg/ptr"
)

func convertMicroVMToModel(spec *types.MicroVMSpec) (*models.MicroVM, error) {
	uid := ""

	if spec.Uid != nil {
		uid = *spec.Uid
	}

	if spec.Kernel == nil {
		// Add the kernel to the spec if it's missing so that we can correctly progress to the validate step
		spec.Kernel = &types.Kernel{}
	}

	vmid, err := models.NewVMID(spec.Id, spec.Namespace, uid)
	if err != nil {
		return nil, fmt.Errorf("creating vmid from spec: %w", err)
	}

	convertedModel := &models.MicroVM{
		ID: *vmid,
		// Labels
		Spec: models.MicroVMSpec{
			Kernel: models.Kernel{
				Image:            models.ContainerImage(spec.Kernel.Image),
				CmdLine:          spec.Kernel.Cmdline,
				AddNetworkConfig: spec.Kernel.AddNetworkConfig,
			},
			VCPU:            int64(spec.Vcpu),
			MemoryInMb:      int64(spec.MemoryInMb),
			Metadata:        instance.New(),
			AllowGuestAgent: spec.AllowGuestAgent,
		},
	}

	if convertedModel.Spec.VCPU == 0 {
		convertedModel.Spec.VCPU = defaults.VCPU
	}

	if convertedModel.Spec.MemoryInMb == 0 {
		convertedModel.Spec.MemoryInMb = defaults.MemoryInMb
	}

	if spec.Kernel.Filename != nil {
		convertedModel.Spec.Kernel.Filename = *spec.Kernel.Filename
	}

	if spec.Initrd != nil {
		convertedModel.Spec.Initrd = &models.Initrd{
			Image: models.ContainerImage(spec.Initrd.Image),
		}
		if spec.Initrd.Filename != nil {
			convertedModel.Spec.Initrd.Filename = *spec.Initrd.Filename
		}
	}

	if spec.RootVolume != nil {
		convertedModel.Spec.RootVolume = *convertVolumeToModel(spec.RootVolume)
	}

	for _, volume := range spec.AdditionalVolumes {
		convertedVolume := convertVolumeToModel(volume)
		convertedModel.Spec.AdditionalVolumes = append(convertedModel.Spec.AdditionalVolumes, *convertedVolume)
	}

	for _, netInt := range spec.Interfaces {
		convertedNetInt := convertNetworkInterfaceToModel(netInt)
		convertedModel.Spec.NetworkInterfaces = append(convertedModel.Spec.NetworkInterfaces, *convertedNetInt)
	}

	convertedModel.Spec.Metadata = map[string]string{}
	for metadataKey, metadataValue := range spec.Metadata {
		convertedModel.Spec.Metadata[metadataKey] = metadataValue
	}

	if spec.Provider != nil {
		convertedModel.Spec.Provider = *spec.Provider
	}

	if spec.CpuConfig != nil {
		convertedModel.Spec.CPUConfig = &models.CPUConfig{
			FeaturesToEnable:         spec.CpuConfig.FeaturesToEnable,
			KVMCapabilitiesToDisable: spec.CpuConfig.KvmCapabilitiesToDisable,
		}
	}

	return convertedModel, nil
}

func convertNetworkInterfaceToModel(netInt *types.NetworkInterface) *models.NetworkInterface {
	converted := &models.NetworkInterface{
		GuestDeviceName:       netInt.DeviceId,
		AllowMetadataRequests: false,
	}

	if netInt.GuestMac != nil {
		converted.GuestMAC = *netInt.GuestMac
	}

	if netInt.Address != nil {
		converted.StaticAddress = &models.StaticAddress{
			Address:     models.IPAddressCIDR(netInt.Address.Address),
			Nameservers: []string{},
		}
		if netInt.Address.Gateway != nil {
			converted.StaticAddress.Gateway = (*models.IPAddressCIDR)(netInt.Address.Gateway)
		}

		converted.StaticAddress.Nameservers = append(converted.StaticAddress.Nameservers, netInt.Address.Nameservers...)
	}
	if netInt.Overrides != nil {
		converted.BridgeName = *netInt.Overrides.BridgeName
	}

	switch netInt.Type {
	case types.NetworkInterface_MACVTAP:
		converted.Type = models.IfaceTypeMacvtap
	case types.NetworkInterface_TAP:
		converted.Type = models.IfaceTypeTap
	}

	return converted
}

func convertVolumeToModel(volume *types.Volume) *models.Volume {
	convertedVol := &models.Volume{
		ID:         volume.Id,
		IsReadOnly: volume.IsReadOnly,
		Mode:       convertVolumeModeToModel(volume.Mode),
	}

	if volume.PartitionId != nil {
		convertedVol.PartitionID = *volume.PartitionId
	}

	if volume.SizeInMb != nil {
		convertedVol.Size = *volume.SizeInMb
	}
	if volume.Source != nil {
		if volume.Source.ContainerSource != nil {
			convertedVol.Source.Container = &models.ContainerVolumeSource{
				Image: models.ContainerImage(*volume.Source.ContainerSource),
			}
		}
		if volume.Source.VirtiofsSource != nil {
			convertedVol.Source.VirtioFS = &models.VirtioFSVolumeSource{
				Path: *volume.Source.VirtiofsSource,
			}
		}
	}

	if volume.MountPoint != nil {
		convertedVol.MountPoint = *volume.MountPoint
	}

	return convertedVol
}

// convertVolumeModeToModel maps the proto volume mode to the model. FULL is the
// proto default, so a request from a client that predates the field is full.
func convertVolumeModeToModel(mode types.VolumeMode) models.VolumeMode {
	if mode == types.VolumeMode_OVERLAY {
		return models.VolumeModeOverlay
	}

	return models.VolumeModeFull
}

func convertModelToVolumeMode(mode models.VolumeMode) types.VolumeMode {
	if mode == models.VolumeModeOverlay {
		return types.VolumeMode_OVERLAY
	}

	return types.VolumeMode_FULL
}

func convertModelToMicroVMSpec(mvm *models.MicroVM) *types.MicroVMSpec {
	converted := &types.MicroVMSpec{
		Id:        mvm.ID.Name(),
		Namespace: mvm.ID.Namespace(),
		Uid:       ptr.String(mvm.ID.UID()),
		// Labels: ,
		Vcpu:            int32(mvm.Spec.VCPU),
		MemoryInMb:      int32(mvm.Spec.MemoryInMb),
		AllowGuestAgent: mvm.Spec.AllowGuestAgent,
		Kernel: &types.Kernel{
			Image:            string(mvm.Spec.Kernel.Image),
			Cmdline:          mvm.Spec.Kernel.CmdLine,
			Filename:         &mvm.Spec.Kernel.Filename,
			AddNetworkConfig: mvm.Spec.Kernel.AddNetworkConfig,
		},
	}

	if mvm.Spec.Initrd != nil {
		converted.Initrd = &types.Initrd{
			Image:    (string)(mvm.Spec.Initrd.Image),
			Filename: &mvm.Spec.Initrd.Filename,
		}
	}

	for i := range mvm.Spec.NetworkInterfaces {
		convertedNetInt := convertModelToNetworkInterface(&mvm.Spec.NetworkInterfaces[i])
		converted.Interfaces = append(converted.Interfaces, convertedNetInt)
	}

	if (models.Volume{}) != mvm.Spec.RootVolume {
		converted.RootVolume = convertModelToVolumne(&mvm.Spec.RootVolume)
	}

	for i := range mvm.Spec.AdditionalVolumes {
		convertedVol := convertModelToVolumne(&mvm.Spec.AdditionalVolumes[i])
		converted.AdditionalVolumes = append(converted.AdditionalVolumes, convertedVol)
	}

	converted.Metadata = map[string]string{}

	for metadataKey, metadataValue := range mvm.Spec.Metadata {
		converted.Metadata[metadataKey] = metadataValue
	}

	if mvm.Spec.CPUConfig != nil {
		converted.CpuConfig = &types.CPUConfig{
			FeaturesToEnable:         mvm.Spec.CPUConfig.FeaturesToEnable,
			KvmCapabilitiesToDisable: mvm.Spec.CPUConfig.KVMCapabilitiesToDisable,
		}
	}

	return converted
}

func convertModelToVolumne(modelVolume *models.Volume) *types.Volume {
	convertedVol := &types.Volume{
		Id:          modelVolume.ID,
		IsReadOnly:  modelVolume.IsReadOnly,
		PartitionId: &modelVolume.PartitionID,
		SizeInMb:    &modelVolume.Size,
		Mode:        convertModelToVolumeMode(modelVolume.Mode),
	}

	if modelVolume.MountPoint != "" {
		convertedVol.MountPoint = &modelVolume.MountPoint
	}

	volumeSource := &types.VolumeSource{}

	if modelVolume.Source.Container != nil {
		volumeSource.ContainerSource = (*string)(&modelVolume.Source.Container.Image)
	}
	if modelVolume.Source.VirtioFS != nil {
		volumeSource.VirtiofsSource = &modelVolume.Source.VirtioFS.Path
	}

	// Assign the populated VolumeSource to the converted Volume
	convertedVol.Source = volumeSource

	return convertedVol
}

func convertModelToNetworkInterface(modelNetInt *models.NetworkInterface) *types.NetworkInterface {
	converted := &types.NetworkInterface{
		GuestMac: &modelNetInt.GuestMAC,
		DeviceId: modelNetInt.GuestDeviceName,
		// HostDevice
	}

	switch modelNetInt.Type {
	case models.IfaceTypeMacvtap:
		converted.Type = types.NetworkInterface_MACVTAP
	case models.IfaceTypeTap:
		converted.Type = types.NetworkInterface_TAP
	case models.IfaceTypeUnsupported:
	}

	if modelNetInt.StaticAddress != nil {
		converted.Address = &types.StaticAddress{
			Address:     string(modelNetInt.StaticAddress.Address),
			Nameservers: []string{},
		}

		if modelNetInt.StaticAddress.Gateway != nil {
			converted.Address.Gateway = (*string)(modelNetInt.StaticAddress.Gateway)
		}

		converted.Address.Nameservers = append(converted.Address.Nameservers, modelNetInt.StaticAddress.Nameservers...)
	}

	return converted
}

func convertModelToMicroVMStatus(mvm *models.MicroVM) *types.MicroVMStatus {
	converted := &types.MicroVMStatus{
		Retry:     int32(mvm.Status.Retry),
		VsockPath: mvm.Status.VSockPath,
	}

	switch mvm.Status.State {
	case models.PendingState:
		converted.State = types.MicroVMStatus_PENDING
	case models.CreatedState:
		converted.State = types.MicroVMStatus_CREATED
	case models.FailedState:
		converted.State = types.MicroVMStatus_FAILED
	case models.DeletingState:
		converted.State = types.MicroVMStatus_DELETING
	}

	converted.Volumes = make(map[string]*types.VolumeStatus, len(mvm.Status.Volumes))
	for volName, volStatus := range mvm.Status.Volumes {
		converted.Volumes[volName] = convertModelToVolumeStatus(volStatus)
	}

	if mvm.Status.KernelMount != nil {
		converted.KernelMount = convertModelToVolumeMount(mvm.Status.KernelMount)
	}

	if mvm.Status.InitrdMount != nil {
		converted.InitrdMount = convertModelToVolumeMount(mvm.Status.InitrdMount)
	}

	converted.NetworkInterfaces = make(map[string]*types.NetworkInterfaceStatus, len(mvm.Status.NetworkInterfaces))
	for netIfaceName, netIfaceStatus := range mvm.Status.NetworkInterfaces {
		converted.NetworkInterfaces[netIfaceName] = convertModelToNetworkInterfaceStatus(netIfaceStatus)
	}

	for _, cond := range mvm.Status.Conditions {
		converted.Conditions = append(converted.Conditions, convertModelToCondition(cond))
	}

	return converted
}

func convertModelToCondition(cond models.Condition) *types.MicroVMCondition {
	converted := &types.MicroVMCondition{
		Type:           string(cond.Type),
		Reason:         cond.Reason,
		Message:        cond.Message,
		LastTransition: timestamppb.New(cond.LastTransition),
	}

	switch cond.Status {
	case models.ConditionTrue:
		converted.Status = types.MicroVMCondition_TRUE
	case models.ConditionFalse:
		converted.Status = types.MicroVMCondition_FALSE
	case models.ConditionUnknown:
		converted.Status = types.MicroVMCondition_UNKNOWN
	}

	return converted
}

func convertModelToVolumeStatus(volStatus *models.VolumeStatus) *types.VolumeStatus {
	converted := &types.VolumeStatus{
		Mount:      convertModelToVolumeMount(&volStatus.Mount),
		BaseDigest: volStatus.BaseDigest,
	}

	if volStatus.BaseMount != nil {
		converted.BaseMount = convertModelToVolumeMount(volStatus.BaseMount)
	}

	return converted
}

func convertModelToVolumeMount(volMount *models.Mount) *types.Mount {
	converted := &types.Mount{
		Source: volMount.Source,
	}

	switch volMount.Type {
	case models.MountTypeDev:
		converted.Type = types.Mount_DEV
	case models.MountTypeHostPath:
		converted.Type = types.Mount_HOSTPATH
	case models.MountTypeFile:
		converted.Type = types.Mount_FILE
	}

	return converted
}

func convertModelToNetworkInterfaceStatus(netStatus *models.NetworkInterfaceStatus) *types.NetworkInterfaceStatus {
	converted := &types.NetworkInterfaceStatus{
		HostDeviceName: netStatus.HostDeviceName,
		Index:          int32(netStatus.Index),
		MacAddress:     netStatus.MACAddress,
	}

	return converted
}

//nolint:unused // the snapshot API is not served yet
func convertSnapshotToModel(snapshot *types.Snapshot) *models.Snapshot {
	converted := &models.Snapshot{
		UID:       snapshot.GetUid(),
		Name:      snapshot.GetName(),
		Namespace: snapshot.GetNamespace(),
		Labels:    maps.Clone(snapshot.GetLabels()),
		Status:    convertSnapshotStatusToModel(snapshot.GetStatus()),
	}

	if source := snapshot.GetSource(); source != nil {
		converted.Source = *models.NewVMIDForce(source.GetName(), source.GetNamespace(), source.GetUid())
	}

	if snapshot.GetCreatedAt() != nil {
		converted.CreatedAt = snapshot.GetCreatedAt().AsTime()
	}

	return converted
}

func convertModelToSnapshot(snapshot *models.Snapshot) *types.Snapshot {
	converted := &types.Snapshot{
		Uid:       snapshot.UID,
		Name:      snapshot.Name,
		Namespace: snapshot.Namespace,
		Source: &types.SourceMicroVM{
			Uid:       snapshot.Source.UID(),
			Name:      snapshot.Source.Name(),
			Namespace: snapshot.Source.Namespace(),
		},
		Labels: maps.Clone(snapshot.Labels),
		Status: convertModelToSnapshotStatus(&snapshot.Status),
	}

	if !snapshot.CreatedAt.IsZero() {
		converted.CreatedAt = timestamppb.New(snapshot.CreatedAt)
	}

	return converted
}

//nolint:unused // the snapshot API is not served yet
func convertSnapshotStatusToModel(status *types.SnapshotStatus) models.SnapshotStatus {
	return models.SnapshotStatus{
		Phase:     convertSnapshotPhaseToModel(status.GetPhase()),
		Error:     status.GetError(),
		LocalPath: status.GetLocalPath(),
	}
}

func convertModelToSnapshotStatus(status *models.SnapshotStatus) *types.SnapshotStatus {
	return &types.SnapshotStatus{
		Phase:     convertModelToSnapshotPhase(status.Phase),
		Error:     status.Error,
		LocalPath: status.LocalPath,
	}
}

//nolint:unused // the snapshot API is not served yet
func convertSnapshotPhaseToModel(phase types.SnapshotStatus_Phase) models.SnapshotPhase {
	switch phase {
	case types.SnapshotStatus_QUIESCING:
		return models.SnapshotPhaseQuiescing
	case types.SnapshotStatus_CAPTURING:
		return models.SnapshotPhaseCapturing
	case types.SnapshotStatus_PACKAGING:
		return models.SnapshotPhasePackaging
	case types.SnapshotStatus_PUSHING:
		return models.SnapshotPhasePushing
	case types.SnapshotStatus_READY:
		return models.SnapshotPhaseReady
	case types.SnapshotStatus_FAILED:
		return models.SnapshotPhaseFailed
	case types.SnapshotStatus_PENDING:
	}

	return models.SnapshotPhasePending
}

func convertModelToSnapshotPhase(phase models.SnapshotPhase) types.SnapshotStatus_Phase {
	switch phase {
	case models.SnapshotPhaseQuiescing:
		return types.SnapshotStatus_QUIESCING
	case models.SnapshotPhaseCapturing:
		return types.SnapshotStatus_CAPTURING
	case models.SnapshotPhasePackaging:
		return types.SnapshotStatus_PACKAGING
	case models.SnapshotPhasePushing:
		return types.SnapshotStatus_PUSHING
	case models.SnapshotPhaseReady:
		return types.SnapshotStatus_READY
	case models.SnapshotPhaseFailed:
		return types.SnapshotStatus_FAILED
	case models.SnapshotPhasePending:
	}

	return types.SnapshotStatus_PENDING
}

func convertSnapshotRefToModel(ref *types.SnapshotRef) models.SnapshotRef {
	converted := models.SnapshotRef{
		UID: ref.GetUid(),
	}

	if named := ref.GetNamed(); named != nil {
		converted.Name = named.GetName()
		converted.Namespace = named.GetNamespace()
	}

	return converted
}

//nolint:unused // the snapshot API is not served yet
func convertModelToSnapshotRef(ref *models.SnapshotRef) *types.SnapshotRef {
	if ref.UID != "" {
		return &types.SnapshotRef{Ref: &types.SnapshotRef_Uid{Uid: ref.UID}}
	}

	return &types.SnapshotRef{
		Ref: &types.SnapshotRef_Named{Named: &types.NamedRef{Name: ref.Name, Namespace: ref.Namespace}},
	}
}

//nolint:unused // the snapshot API is not served yet
func convertSnapshotSourceToModel(source *types.SnapshotSource) *models.SnapshotSource {
	converted := &models.SnapshotSource{
		Snapshot:   convertSnapshotRefToModel(source.GetSnapshot()),
		Name:       source.GetName(),
		Namespace:  source.GetNamespace(),
		UID:        source.GetUid(),
		Labels:     maps.Clone(source.GetLabels()),
		Metadata:   maps.Clone(source.GetMetadata()),
		MemoryMode: convertMemoryModeToModel(source.GetMemoryMode()),
		Force:      source.GetForce(),
	}

	for _, override := range source.GetNetworkOverrides() {
		converted.NetworkOverrides = append(converted.NetworkOverrides, models.HostNetworkOverride{
			GuestDeviceName: override.GetGuestDeviceName(),
			HostDeviceName:  override.GetHostDeviceName(),
		})
	}

	return converted
}

//nolint:unused // the snapshot API is not served yet
func convertModelToSnapshotSource(source *models.SnapshotSource) *types.SnapshotSource {
	converted := &types.SnapshotSource{
		Snapshot:   convertModelToSnapshotRef(&source.Snapshot),
		Name:       source.Name,
		Labels:     maps.Clone(source.Labels),
		Metadata:   maps.Clone(source.Metadata),
		MemoryMode: convertModelToMemoryMode(source.MemoryMode),
		Force:      source.Force,
	}

	if source.Namespace != "" {
		converted.Namespace = ptr.String(source.Namespace)
	}

	if source.UID != "" {
		converted.Uid = ptr.String(source.UID)
	}

	for _, override := range source.NetworkOverrides {
		converted.NetworkOverrides = append(converted.NetworkOverrides, &types.HostNetworkOverride{
			GuestDeviceName: override.GuestDeviceName,
			HostDeviceName:  override.HostDeviceName,
		})
	}

	return converted
}

//nolint:unused // the snapshot API is not served yet
func convertMemoryModeToModel(mode types.MemoryMode) models.MemoryMode {
	switch mode {
	case types.MemoryMode_FILE:
		return models.MemoryModeFile
	case types.MemoryMode_COPY:
		return models.MemoryModeCopy
	case types.MemoryMode_MEMORY_MODE_UNSPECIFIED:
	}

	return models.MemoryModeDefault
}

//nolint:unused // the snapshot API is not served yet
func convertModelToMemoryMode(mode models.MemoryMode) types.MemoryMode {
	switch mode {
	case models.MemoryModeFile:
		return types.MemoryMode_FILE
	case models.MemoryModeCopy:
		return types.MemoryMode_COPY
	case models.MemoryModeDefault:
	}

	return types.MemoryMode_MEMORY_MODE_UNSPECIFIED
}
