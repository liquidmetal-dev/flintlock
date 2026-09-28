package cloudinit

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"github.com/liquidmetal-dev/flintlock/client/cloudinit"
	"github.com/liquidmetal-dev/flintlock/client/cloudinit/userdata"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
	"github.com/liquidmetal-dev/flintlock/pkg/planner"
)

var errNoDriveForVolume = errors.New("provider presents no drive for the volume")

func NewDiskMountStep(vm *models.MicroVM, provider ports.MicroVMService) planner.Procedure {
	return &diskMountStep{
		vm:       vm,
		provider: provider,
	}
}

type diskMountStep struct {
	vm       *models.MicroVM
	provider ports.MicroVMService
}

// mountableVolumes returns the additional volumes that are block devices and
// have a mount point. A virtiofs volume is not a block device, so cloud-init
// cannot mount it by device name.
func (s *diskMountStep) mountableVolumes() []models.Volume {
	volumes := []models.Volume{}

	for _, vol := range s.vm.Spec.AdditionalVolumes {
		if vol.MountPoint == "" || vol.Source.VirtioFS != nil {
			continue
		}

		volumes = append(volumes, vol)
	}

	return volumes
}

// Name is the name of the procedure/operation.
func (s *diskMountStep) Name() string {
	return "cloudinit_disk_mount"
}

func (s *diskMountStep) ShouldDo(ctx context.Context) (bool, error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step": s.Name(),
	})
	logger.Debug("checking if procedure should be run")

	volumes := s.mountableVolumes()
	if len(volumes) == 0 {
		return false, nil
	}

	for _, vol := range volumes {
		status := s.vm.Status.Volumes[vol.ID]

		if status == nil || status.Mount.Source == "" {
			return true, nil
		}
	}

	vendorData, err := s.getVendorData()
	if err != nil {
		return false, fmt.Errorf("getting vendor data: %w", err)
	}

	if vendorData == nil {
		return true, nil
	}

	for _, vol := range volumes {
		if !vendorData.HasMountByMountPoint(vol.MountPoint) {
			return true, nil
		}
	}

	return false, nil
}

// Do will perform the operation/procedure.
func (s *diskMountStep) Do(ctx context.Context) ([]planner.Procedure, error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step": s.Name(),
	})
	logger.Debug("running step to mount additional disks via cloud-init")

	vendorData, err := s.getVendorData()
	if err != nil {
		return nil, fmt.Errorf("getting vendor data: %w", err)
	}
	if vendorData == nil {
		vendorData = &userdata.UserData{}
	}

	drives, err := s.provider.Drives(s.vm)
	if err != nil {
		return nil, fmt.Errorf("getting drives from provider: %w", err)
	}

	for _, vol := range s.mountableVolumes() {
		device, found := drives.GuestDeviceForVolume(vol.ID)
		if !found {
			return nil, fmt.Errorf("volume %s: %w", vol.ID, errNoDriveForVolume)
		}

		// A mount point that is already there is left as it is, so that a
		// microvm created with other device names is not changed.
		if !vendorData.HasMountByMountPoint(vol.MountPoint) {
			vendorData.Mounts = append(vendorData.Mounts, userdata.Mount{
				device,
				vol.MountPoint,
			})
		}
	}
	vendorData.MountDefaultFields = userdata.Mount{"None", "None", "auto", "defaults,nofail", "0", "2"}

	data, err := yaml.Marshal(vendorData)
	if err != nil {
		return nil, fmt.Errorf("marshalling vendor-data to yaml: %w", err)
	}
	dataWithHeader := append([]byte("## template: jinja\n#cloud-config\n\n"), data...)

	if s.vm.Spec.Metadata == nil {
		s.vm.Spec.Metadata = map[string]string{}
	}
	s.vm.Spec.Metadata[cloudinit.VendorDataKey] = base64.StdEncoding.EncodeToString(dataWithHeader)

	return nil, nil
}

func (s *diskMountStep) Verify(_ context.Context) error {
	return nil
}

func (s *diskMountStep) getVendorData() (*userdata.UserData, error) {
	vendorDataRaw, ok := s.vm.Spec.Metadata[cloudinit.VendorDataKey]
	if !ok {
		return nil, nil
	}

	vendorData := &userdata.UserData{}
	data, err := base64.StdEncoding.DecodeString(vendorDataRaw)
	if err != nil {
		return nil, fmt.Errorf("decoding vendor data: %w", err)
	}
	if marshalErr := yaml.Unmarshal(data, vendorData); marshalErr != nil {
		return nil, fmt.Errorf("unmarshalling vendor-data yaml: %w", err)
	}

	return vendorData, nil
}
