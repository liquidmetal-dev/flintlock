package cloudhypervisor

import (
	"context"
	"fmt"

	"github.com/weaveworks-liquidmetal/flintlock/client/cloudinit"
	"github.com/weaveworks-liquidmetal/flintlock/core/models"
	"github.com/weaveworks-liquidmetal/flintlock/core/ports"
	"github.com/weaveworks-liquidmetal/flintlock/infrastructure/microvm/shared"
)

func (p *provider) createCloudInitImage(ctx context.Context, vm *models.MicroVM, state State) error {
	imagePath := state.CloudInitImage()

	if vm.Spec.Kernel.AddNetworkConfig {
		networkConfig, err := shared.GenerateNetworkConfig(vm)
		if err != nil {
			return fmt.Errorf("generating kernel network-config: %w", err)
		}
		vm.Spec.Metadata["network-config"] = networkConfig
	}

	files := []ports.DiskFile{}
	for k, v := range vm.Spec.Metadata {
		cloudInitKey := isCloudInitKey(k)
		if !cloudInitKey {
			continue
		}

		dest := fmt.Sprintf("/%s", k)
		files = append(files, ports.DiskFile{
			Path:          dest,
			ContentBase64: v,
		})
	}

	input := ports.DiskCreateInput{
		Path:       imagePath,
		Size:       "8Mb",
		VolumeName: cloudinit.VolumeName,
		Type:       ports.DiskTypeFat32,
		Overwrite:  true,
		Files:      files,
	}
	if err := p.diskSvc.Create(ctx, input); err != nil {
		return fmt.Errorf("creating cloud-init volume %s: %w", imagePath, err)
	}

	return nil
}

func isCloudInitKey(keyName string) bool {
	switch keyName {
	case cloudinit.InstanceDataKey:
		return true
	case cloudinit.NetworkConfigDataKey:
		return true
	case cloudinit.UserdataKey:
		return true
	case cloudinit.VendorDataKey:
		return true
	default:
		return false
	}
}
