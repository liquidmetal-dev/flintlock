package microvm

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/weaveworks-liquidmetal/flintlock/core/models"
	"github.com/weaveworks-liquidmetal/flintlock/core/ports"
	"github.com/weaveworks-liquidmetal/flintlock/core/steps/runtime"
	"github.com/weaveworks-liquidmetal/flintlock/pkg/log"
	"github.com/weaveworks-liquidmetal/flintlock/pkg/planner"
)

// GetContentFunc is a function type that is used to get the disk content.
type GetContentFunc func() map[string]string

type CreateVolumeDiskStepInput struct {
	VM          *models.MicroVM
	DiskSvc     ports.DiskService
	FS          afero.Fs
	VolumeID    string
	VolumeSize  string
	ContentFunc GetContentFunc
}

func NewCreateVolumeDiskStep(input *CreateVolumeDiskStepInput) planner.Procedure {
	return &createVolumeDiskStep{
		vm:             input.VM,
		volumeID:       input.VolumeID,
		volumeSize:     input.VolumeSize,
		getContentFunc: input.ContentFunc,

		diskSvc: input.DiskSvc,
		fs:      input.FS,
	}
}

type createVolumeDiskStep struct {
	vm             *models.MicroVM
	volumeID       string
	volumeSize     string
	getContentFunc GetContentFunc

	diskSvc ports.DiskService
	fs      afero.Fs
}

// Name is the name of the procedure/operation.
func (s *createVolumeDiskStep) Name() string {
	return "microvm_create_volume_disk"
}

func (s *createVolumeDiskStep) ShouldDo(ctx context.Context) (bool, error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step":      s.Name(),
		"vmid":      s.vm.ID,
		"volume_id": s.volumeID,
	})
	logger.Debug("checking if procedure should be run")

	return true, nil
}

// Do will perform the operation/procedure.
func (s *createVolumeDiskStep) Do(ctx context.Context) ([]planner.Procedure, error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step":      s.Name(),
		"vmid":      s.vm.ID,
		"volume_id": s.volumeID,
	})
	logger.Debug("creating disk for volume")

	vol := s.vm.Spec.AdditionalVolumes.GetByID(s.volumeID)
	if vol == nil {
		return nil, fmt.Errorf("couldn't find volume with id %s", s.volumeID)
	}

	files := []ports.DiskFile{}
	for k, v := range s.getContentFunc() {
		files = append(files, ports.DiskFile{
			Path:          fmt.Sprintf("/%s", k),
			ContentBase64: v,
		})
	}

	step := runtime.NewDiskCreateStep(&runtime.DiskCreateStepInput{
		Path:           vol.Source.HostPath.Path,
		VolumeName:     vol.ID,
		Size:           s.volumeSize,
		DiskType:       ports.DiskTypeFat32,
		Content:        files,
		AlwaysRecreate: false,
	}, s.diskSvc, s.fs)

	return []planner.Procedure{step}, nil

}

func (s *createVolumeDiskStep) Verify(ctx context.Context) error {
	return nil
}
