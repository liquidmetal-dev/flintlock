package metadata_test

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	g "github.com/onsi/gomega"
	"github.com/spf13/afero"

	"github.com/weaveworks-liquidmetal/flintlock/core/models"
	"github.com/weaveworks-liquidmetal/flintlock/core/steps/metadata"
	"github.com/weaveworks-liquidmetal/flintlock/infrastructure/mock"
)

func TestDiskAttachStep_DataDisk(t *testing.T) {
	g.RegisterTestingT(t)

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	diskSvc := mock.NewMockDiskService(mockCtrl)
	ctx := context.Background()

	step := metadata.NewDiskAttachStep(metadata.DiskAttachInput{
		VM:                createMicrovm(),
		DiskSvc:           diskSvc,
		MetadataFilter:    metadata.NotCloudInitFilter,
		FS:                afero.NewMemMapFs(),
		VolumeFileName:    "test.img",
		VolumeName:        "data",
		VolumeSize:        "8Mb",
		VolumeInsertFirst: false,
		CloudInitAttach:   false,
	})

	shouldDo, err := step.ShouldDo(ctx)
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(shouldDo).To(g.BeTrue())

	childSteps, err := step.Do(ctx)
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(childSteps).To(g.HaveLen(2))
}

func TestDiskAttachStep_DiskAlreadyExists(t *testing.T) {
	g.RegisterTestingT(t)

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	diskSvc := mock.NewMockDiskService(mockCtrl)
	ctx := context.Background()
	fs := afero.NewMemMapFs()

	_, err := fs.Create("/tmp/vm/1234/test.img")
	g.Expect(err).NotTo(g.HaveOccurred())

	step := metadata.NewDiskAttachStep(metadata.DiskAttachInput{
		VM:                createMicrovm(),
		DiskSvc:           diskSvc,
		MetadataFilter:    metadata.NotCloudInitFilter,
		FS:                fs,
		VolumeFileName:    "test.img",
		VolumeName:        "data",
		VolumeSize:        "8Mb",
		VolumeInsertFirst: false,
		CloudInitAttach:   false,
	})

	shouldDo, err := step.ShouldDo(ctx)
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(shouldDo).To(g.BeTrue())

	childSteps, err := step.Do(ctx)
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(childSteps).To(g.HaveLen(1))
}

func TestDiskAttachStep_DiskAndAddVolExist(t *testing.T) {
	g.RegisterTestingT(t)

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	diskSvc := mock.NewMockDiskService(mockCtrl)
	ctx := context.Background()
	fs := afero.NewMemMapFs()

	vm := createMicrovm()
	vm.Spec.AdditionalVolumes = append(vm.Spec.AdditionalVolumes, models.Volume{
		ID:         "data",
		IsReadOnly: false,
		Source: models.VolumeSource{
			HostPath: &models.HostPathVolumeSource{
				Path: "/tmp/vm/1234/test.img",
			},
		},
	})

	_, err := fs.Create("/tmp/vm/1234/test.img")
	g.Expect(err).NotTo(g.HaveOccurred())

	step := metadata.NewDiskAttachStep(metadata.DiskAttachInput{
		VM:                vm,
		DiskSvc:           diskSvc,
		MetadataFilter:    metadata.NotCloudInitFilter,
		FS:                fs,
		VolumeFileName:    "test.img",
		VolumeName:        "data",
		VolumeSize:        "8Mb",
		VolumeInsertFirst: false,
		CloudInitAttach:   false,
	})

	shouldDo, err := step.ShouldDo(ctx)
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(shouldDo).To(g.BeFalse())
}

func createMicrovm() *models.MicroVM {
	vmid, _ := models.NewVMID("vm", "ns", "uid")
	return &models.MicroVM{
		ID:      *vmid,
		Version: 1,
		Spec: models.MicroVMSpec{
			Metadata: models.Metadata{
				Items:     map[string]string{},
				AddVolume: false,
			},
		},
		Status: models.MicroVMStatus{
			RuntimeStateDir: "/tmp/vm/1234",
		},
	}
}
