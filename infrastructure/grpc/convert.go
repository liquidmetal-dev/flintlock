package grpc

import (
	"fmt"

	"github.com/weaveworks/reignite/api/types"
	"github.com/weaveworks/reignite/core/models"
)

func convertMicroVMToModel(spec *types.MicroVMSpec) (*models.MicroVM, error) {
	vmid, err := models.NewVMID(spec.Id, spec.Namespace)
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
		},
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

	for _, volume := range spec.Volumes {
		convertedVolume := convertVolumeToModel(volume)
		convertedModel.Spec.Volumes = append(convertedModel.Spec.Volumes, *convertedVolume)
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
		AllowMetadataRequests: netInt.AllowMetadataReq,
		GuestDeviceName:       netInt.GuestDeviceName,
	}
	if netInt.GuestMac != nil {
		converted.GuestMAC = *netInt.GuestMac
	}
	if netInt.Address != nil {
		converted.Address = *netInt.Address
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
		MountPoint: volume.MountPoint,
		IsRoot:     volume.IsRoot,
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

	return convertedVol
}

func convertModelToMicroVM(mvm *models.MicroVM) *types.MicroVMSpec {
	converted := &types.MicroVMSpec{
		Id:        mvm.ID.Name(),
		Namespace: mvm.ID.Namespace(),
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

	for i := range mvm.Spec.Volumes {
		convertedVol := convertModelToVolumne(&mvm.Spec.Volumes[i])
		converted.Volumes = append(converted.Volumes, convertedVol)
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
		MountPoint:  modelVolume.MountPoint,
		IsRoot:      modelVolume.IsRoot,
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
		AllowMetadataReq: modelNetInt.AllowMetadataRequests,
		GuestMac:         &modelNetInt.GuestMAC,
		GuestDeviceName:  modelNetInt.GuestDeviceName,
		// HostDevice
	}

	switch modelNetInt.Type {
	case models.IfaceTypeMacvtap:
		converted.Type = types.NetworkInterface_MACVTAP
	case models.IfaceTypeTap:
		converted.Type = types.NetworkInterface_TAP
	}

	if modelNetInt.Address != "" {
		converted.Address = &modelNetInt.Address
	}

	return converted
}
