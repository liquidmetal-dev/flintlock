package microvm

import (
	"context"

	"github.com/sirupsen/logrus"
	"github.com/weaveworks-liquidmetal/flintlock/core/models"
	"github.com/weaveworks-liquidmetal/flintlock/pkg/log"
	"github.com/weaveworks-liquidmetal/flintlock/pkg/planner"
)

func NewAttachVolumeStep(vm *models.MicroVM, path, name string, insertFirst, readonly bool) planner.Procedure {
	return &attachVolumeStep{
		vm:          vm,
		path:        path,
		name:        name,
		insertFirst: insertFirst,
		readonly:    readonly,
	}
}

type attachVolumeStep struct {
	vm          *models.MicroVM
	path        string
	name        string
	insertFirst bool
	readonly    bool
}

// Name is the name of the procedure/operation.
func (s *attachVolumeStep) Name() string {
	return "microvm_attach_volume"
}

func (s *attachVolumeStep) ShouldDo(ctx context.Context) (bool, error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step":   s.Name(),
		"vmid":   s.vm.ID,
		"volume": s.name,
	})
	logger.Debug("checking if procedure should be run")

	for _, volume := range s.vm.Spec.AdditionalVolumes {
		if volume.ID == s.name {
			return false, nil
		}
	}

	return true, nil
}

// Do will perform the operation/procedure.
func (s *attachVolumeStep) Do(ctx context.Context) ([]planner.Procedure, error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step":   s.Name(),
		"vmid":   s.vm.ID,
		"volume": s.name,
	})
	logger.Debug("attaching additional volume to microvm")

	if s.vm.Spec.AdditionalVolumes == nil {
		s.vm.Spec.AdditionalVolumes = models.Volumes{}
	}

	vol := models.Volume{
		ID:         s.name,
		IsReadOnly: s.readonly,
		Source: models.VolumeSource{
			HostPath: &models.HostPathVolumeSource{
				Path: s.path,
			},
		},
	}

	if s.insertFirst {
		s.vm.Spec.AdditionalVolumes = append([]models.Volume{vol}, s.vm.Spec.AdditionalVolumes...)
	} else {
		s.vm.Spec.AdditionalVolumes = append(s.vm.Spec.AdditionalVolumes, vol)
	}

	if s.vm.Status.Volumes == nil {
		s.vm.Status.Volumes = models.VolumeStatuses{}
	}

	s.vm.Status.Volumes[vol.ID] = &models.VolumeStatus{
		Mount: models.Mount{
			Type:   models.MountTypeHostPath,
			Source: s.path,
		},
	}

	return nil, nil
}

func (s *attachVolumeStep) Verify(ctx context.Context) error {
	return nil
}
