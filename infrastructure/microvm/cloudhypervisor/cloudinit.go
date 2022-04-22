package cloudhypervisor

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"

	"github.com/diskfs/go-diskfs"
	"github.com/diskfs/go-diskfs/disk"
	"github.com/diskfs/go-diskfs/filesystem"

	"github.com/weaveworks-liquidmetal/flintlock/client/cloudinit"
	"github.com/weaveworks-liquidmetal/flintlock/core/models"
	"github.com/weaveworks-liquidmetal/flintlock/infrastructure/microvm/shared"
)

func (p *provider) createCloudInitImage(vm *models.MicroVM, state State) error {
	imagePath := state.CloudInitImage()

	if _, err := p.fs.Stat(imagePath); err == nil {
		if removeErr := p.fs.Remove(imagePath); removeErr != nil {
			return fmt.Errorf("removing cloud-init image %s: %w", imagePath, removeErr)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("checking cloud-init image exists: %w", err)
	}

	diskSize := 8192 * 1024 * 1024 // 8192 MB

	metaDisk, err := diskfs.Create(imagePath, int64(diskSize), diskfs.Raw)
	if err != nil {
		return fmt.Errorf("creating image file %s: %w", imagePath, err)
	}

	metaDisk.LogicalBlocksize = 512
	fspec := disk.FilesystemSpec{
		Partition:   0,
		FSType:      filesystem.TypeFat32,
		VolumeLabel: cloudinit.VolumeName,
	}
	fs, err := metaDisk.CreateFilesystem(fspec)
	if err != nil {
		return fmt.Errorf("creating FAT filesystem on %s: %w", imagePath, err)
	}

	if vm.Spec.Kernel.AddNetworkConfig {
		networkConfig, err := shared.GenerateNetworkConfig(vm)
		if err != nil {
			return fmt.Errorf("generating kernel network-config: %w", err)
		}
		vm.Spec.Metadata["network-config"] = networkConfig
	}

	for k, v := range vm.Spec.Metadata {
		cloudInitKey := isCloudInitKey(k)
		if !cloudInitKey {
			//TODO:  debug log we are ignoring
			continue
		}

		dest := fmt.Sprintf("/%s", k)
		if writeErr := createFileInImage(dest, v, fs); writeErr != nil {
			return fmt.Errorf("creating file %s in image: %w", dest, writeErr)
		}
	}

	return nil

}

func createFileInImage(dest string, content string, fs filesystem.FileSystem) error {
	decoded, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return fmt.Errorf("base64 decoding content %s: %w", content, err)
	}

	rw, err := fs.OpenFile(dest, os.O_CREATE|os.O_RDWR)
	if err != nil {
		return err
	}

	_, err = rw.Write(decoded)
	if err != nil {
		return err
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
