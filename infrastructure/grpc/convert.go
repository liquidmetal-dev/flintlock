package grpc

import (
	"fmt"

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
			VCPU:       int64(spec.Vcpu),
			MemoryInMb: int64(spec.MemoryInMb),
			Metadata:   instance.New(),
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

		for index := range netInt.Address.Nameservers {
			nameserver := netInt.Address.Nameservers[index]
			converted.StaticAddress.Nameservers = append(converted.StaticAddress.Nameservers, nameserver)
		}
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
	}

	if volume.MountPoint != nil {
		convertedVol.MountPoint = *volume.MountPoint
	}

	return convertedVol
}

func convertModelToMicroVMSpec(mvm *models.MicroVM) *types.MicroVMSpec {
	converted := &types.MicroVMSpec{
		Id:        mvm.ID.Name(),
		Namespace: mvm.ID.Namespace(),
		Uid:       ptr.String(mvm.ID.UID()),
		// Labels: ,
		Vcpu:       int32(mvm.Spec.VCPU),
		MemoryInMb: int32(mvm.Spec.MemoryInMb),
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

	return converted
}

func convertModelToVolumne(modelVolume *models.Volume) *types.Volume {
	convertedVol := &types.Volume{
		Id:          modelVolume.ID,
		IsReadOnly:  modelVolume.IsReadOnly,
		PartitionId: &modelVolume.PartitionID,
		SizeInMb:    &modelVolume.Size,
	}

	if modelVolume.Source.Container != nil {
		convertedVol.Source = &types.VolumeSource{
			ContainerSource: (*string)(&modelVolume.Source.Container.Image),
		}
	}

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

		for index := range modelNetInt.StaticAddress.Nameservers {
			nameserver := modelNetInt.StaticAddress.Nameservers[index]
			converted.Address.Nameservers = append(converted.Address.Nameservers, nameserver)
		}
	}

	return converted
}

func convertModelToMicroVMStatus(mvm *models.MicroVM) *types.MicroVMStatus {
	converted := &types.MicroVMStatus{
		Retry: int32(mvm.Status.Retry),
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

	return converted
}

func convertModelToVolumeStatus(volStatus *models.VolumeStatus) *types.VolumeStatus {
	converted := &types.VolumeStatus{
		Mount: convertModelToVolumeMount(&volStatus.Mount),
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
