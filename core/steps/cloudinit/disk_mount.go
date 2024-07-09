package cloudinit

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"github.com/liquidmetal-dev/flintlock/client/cloudinit"
	"github.com/liquidmetal-dev/flintlock/client/cloudinit/userdata"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
	"github.com/liquidmetal-dev/flintlock/pkg/planner"
)

func NewDiskMountStep(vm *models.MicroVM) planner.Procedure {
	return &diskMountStep{
		vm: vm,
	}
}

type diskMountStep struct {
	vm *models.MicroVM
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

	if !s.vm.Spec.AdditionalVolumes.HasMountableVolumes() {
		return false, nil
	}

	for _, vol := range s.vm.Spec.AdditionalVolumes {
		if vol.MountPoint == "" {
			continue
		}

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

	for _, vol := range s.vm.Spec.AdditionalVolumes {
		if vol.MountPoint == "" {
			continue
		}

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

	startingCode := int('b')
	for i, vol := range s.vm.Spec.AdditionalVolumes {
		if vol.MountPoint == "" {
			continue
		}

		device := fmt.Sprintf("vd%c", rune(startingCode+i)) // Device number is always +1 as we have the root volume first

		if !vendorData.HasMountByName(device) {
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

func (s *diskMountStep) Verify(ctx context.Context) error {
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
