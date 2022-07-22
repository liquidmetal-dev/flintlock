package metadata

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"

	"github.com/weaveworks-liquidmetal/flintlock/core/models"
	"github.com/weaveworks-liquidmetal/flintlock/core/ports"
	"github.com/weaveworks-liquidmetal/flintlock/core/steps/cloudinit"
	"github.com/weaveworks-liquidmetal/flintlock/core/steps/microvm"
	"github.com/weaveworks-liquidmetal/flintlock/core/steps/runtime"
	"github.com/weaveworks-liquidmetal/flintlock/pkg/log"
	"github.com/weaveworks-liquidmetal/flintlock/pkg/planner"
)

// MetadataFilter is a function type that is used to filter metadata items.
type MetadataFilterFunc func(key, name string) bool

// DiskAttachStepInput is the input for creating a new Disk Attach Step.
type DiskAttachInput struct {
	VM                *models.MicroVM
	MetadataFilter    MetadataFilterFunc
	DiskSvc           ports.DiskService
	FS                afero.Fs
	VolumeFileName    string
	VolumeName        string
	VolumeSize        string
	VolumeInsertFirst bool
	CloudInitAttach   bool
}

func NewDiskAttachStep(input DiskAttachInput) planner.Procedure {
	return &diskAttachStep{
		vm:         input.VM,
		filterFunc: input.MetadataFilter,

		volumeFileName:    input.VolumeFileName,
		volumeName:        input.VolumeName,
		volumeSize:        input.VolumeSize,
		volumeInsertFirst: input.VolumeInsertFirst,
		cloudInitAttach:   input.CloudInitAttach,
		diskSvc:           input.DiskSvc,
		fs:                input.FS,
	}
}

type diskAttachStep struct {
	vm         *models.MicroVM
	filterFunc MetadataFilterFunc

	volumeFileName    string
	volumeName        string
	volumeSize        string
	volumeInsertFirst bool
	cloudInitAttach   bool

	fs      afero.Fs
	diskSvc ports.DiskService
}

// Name is the name of the procedure/operation.
func (s *diskAttachStep) Name() string {
	return "metadata_disk_attach"
}

func (s *diskAttachStep) ShouldDo(ctx context.Context) (bool, error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step": s.Name(),
	})
	logger.Debug("checking if procedure should be run")

	if s.vm.Status.RuntimeStateDir == "" {
		return true, nil
	}

	count := 0
	for _, childStep := range s.createChildSteps() {
		shouldDo, err := childStep.ShouldDo(ctx)
		if err != nil {
			return false, fmt.Errorf("checking ShouldDo for child step %s: %w", childStep.Name(), err)
		}
		if shouldDo {
			count++
		}
	}

	return count > 0, nil
}

// Do will perform the operation/procedure.
func (s *diskAttachStep) Do(ctx context.Context) ([]planner.Procedure, error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step": s.Name(),
	})
	logger.Debug("running step to attach a cloud-init disk")

	stepsToRun := []planner.Procedure{}
	for _, childStep := range s.createChildSteps() {
		shouldDo, err := childStep.ShouldDo(ctx)
		if err != nil {
			return nil, fmt.Errorf("checking ShouldDo for child step %s: %w", childStep.Name(), err)
		}
		if shouldDo {
			stepsToRun = append(stepsToRun, childStep)
		}
	}
	if len(stepsToRun) == 0 {
		return nil, nil
	}

	return stepsToRun, nil
}

func (s *diskAttachStep) Verify(ctx context.Context) error {
	return nil
}

func (s *diskAttachStep) createChildSteps() []planner.Procedure {
	imagePath := fmt.Sprintf("%s/%s", s.vm.Status.RuntimeStateDir, s.volumeFileName)
	steps := []planner.Procedure{
		runtime.NewDiskCreateStep(&runtime.DiskCreateStepInput{
			Path:           imagePath,
			VolumeName:     s.volumeName,
			Size:           s.volumeSize,
			DiskType:       ports.DiskTypeFat32,
			Content:        s.getDiskContent(s.vm),
			AlwaysRecreate: false,
		}, s.diskSvc, s.fs),
		microvm.NewAttachVolumeStep(
			s.vm,
			imagePath,
			s.volumeName,
			s.volumeInsertFirst,
			true,
		),
	}
	if s.cloudInitAttach {
		steps = append(steps, cloudinit.NewDiskMountStep(s.vm, "vdb2", "/opt/data"))
	}

	return steps
}

func (s *diskAttachStep) getDiskContent(vm *models.MicroVM) []ports.DiskFile {
	files := []ports.DiskFile{}

	for k, v := range vm.Spec.Metadata.Items {
		if s.filterFunc(k, v) {
			files = append(files, ports.DiskFile{
				Path:          fmt.Sprintf("/%s", k),
				ContentBase64: v,
			})
		}
	}

	return files
}
