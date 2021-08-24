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
		ID: vmid,
		// Labels
		Spec: models.MicroVMSpec{
			Kernel: models.Kernel{
				Image:    models.ContainerImage(spec.Kernel.Image),
				Filename: *spec.Kernel.Filename,
				CmdLine:  spec.Kernel.Cmdline,
			},
			InitrdImage: models.ContainerImage(*spec.InitrdImage),
			VCPU:        int64(spec.Vcpu),
			MemoryInMb:  int64(spec.MemoryInMb),
		},
	}

	for _, volume := range spec.Volumes {
		convertedVolume := convertVolumeToModel(volume)
		convertedModel.Spec.Volumes = append(convertedModel.Spec.Volumes, *convertedVolume)
	}

	for _, netInt := range spec.Interfaces {
		convertedNetInt := convertNetworkInterfaceToModel(netInt)
		convertedModel.Spec.NetworkInterfaces = append(convertedModel.Spec.NetworkInterfaces, *convertedNetInt)
	}

	return convertedModel, nil
}

func convertNetworkInterfaceToModel(netInt *types.NetworkInterface) *models.NetworkInterface {
	return &models.NetworkInterface{
		AllowMetadataRequests: netInt.AllowMetadataReq,
		GuestMAC:              *netInt.GuestMac,
		GuestDeviceName:       *netInt.GuestDeviceName,
	}
}

func convertVolumeToModel(volume *types.Volume) *models.Volume {
	convertedVol := &models.Volume{
		ID:          volume.Id,
		MountPoint:  volume.MountPoint,
		IsRoot:      volume.IsRoot,
		IsReadOnly:  volume.IsReadOnly,
		PartitionID: *volume.PartitionId,
		Size:        *volume.SizeInMb,
	}

	if volume.Source != nil {
		if volume.Source.ContainerSource != nil {
			convertedVol.Source.Container.Image = models.ContainerImage(*volume.Source.ContainerSource)
		}
		if volume.Source.HostpathSource != nil {
			convertedVol.Source.HostPath = &models.HostPathVolumeSource{
				Path: volume.Source.HostpathSource.Path,
			}
			// TODO: in the future change to switch when there are more types.
			if volume.Source.HostpathSource.Type == types.HostPathVolumeSource_RAW_FILE {
				convertedVol.Source.HostPath.Type = models.HostPathRawFile
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
			Image:    string(mvm.Spec.Kernel.Image),
			Cmdline:  mvm.Spec.Kernel.CmdLine,
			Filename: &mvm.Spec.Kernel.Filename,
		},
		InitrdImage: (*string)(&mvm.Spec.InitrdImage),
	}

	for i := range mvm.Spec.NetworkInterfaces {
		convertedNetInt := convertModelToNetworkInterface(&mvm.Spec.NetworkInterfaces[i])
		converted.Interfaces = append(converted.Interfaces, convertedNetInt)
	}

	for i := range mvm.Spec.Volumes {
		convertedVol := convertModelToVolumne(&mvm.Spec.Volumes[i])
		converted.Volumes = append(converted.Volumes, convertedVol)
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
		convertedVol.Source.ContainerSource = (*string)(&modelVolume.Source.Container.Image)
	}
	if modelVolume.Source.HostPath != nil {
		convertedVol.Source.HostpathSource = &types.HostPathVolumeSource{
			Path: modelVolume.Source.HostPath.Path,
		}
		// TODO: in the future change to switch when there are different types
		if modelVolume.Source.HostPath.Type == models.HostPathRawFile {
			convertedVol.Source.HostpathSource.Type = types.HostPathVolumeSource_RAW_FILE
		}
	}

	return convertedVol
}

func convertModelToNetworkInterface(modelNetInt *models.NetworkInterface) *types.NetworkInterface {
	return &types.NetworkInterface{
		AllowMetadataReq: modelNetInt.AllowMetadataRequests,
		GuestMac:         &modelNetInt.GuestMAC,
		GuestDeviceName:  &modelNetInt.GuestDeviceName,
		// HostDevice
	}
}
