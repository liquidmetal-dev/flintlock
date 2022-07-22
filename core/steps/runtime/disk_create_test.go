package runtime_test

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	g "github.com/onsi/gomega"
	"github.com/spf13/afero"
	"github.com/weaveworks-liquidmetal/flintlock/core/ports"
	"github.com/weaveworks-liquidmetal/flintlock/core/steps/runtime"
	"github.com/weaveworks-liquidmetal/flintlock/infrastructure/mock"
)

func TestDiskCreate_ShouldDo(t *testing.T) {
	g.RegisterTestingT(t)

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	diskSvc := mock.NewMockDiskService(mockCtrl)
	ctx := context.Background()
	fs := afero.NewMemMapFs()

	input := runtime.DiskCreateStepInput{
		Path:           "/tmp/test.img",
		VolumeName:     "data",
		Size:           "8Mb",
		DiskType:       ports.DiskTypeFat32,
		Content:        []ports.DiskFile{},
		AlwaysRecreate: false,
	}

	// No existing disk
	step := runtime.NewDiskCreateStep(&input, diskSvc, fs)
	shouldDo, err := step.ShouldDo(ctx)
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(shouldDo).To(g.BeTrue(), "with no existing disk we should do the step")

	// Existing disk
	fs.Create("/tmp/test.img")
	step = runtime.NewDiskCreateStep(&input, diskSvc, fs)
	shouldDo, err = step.ShouldDo(ctx)
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(shouldDo).To(g.BeFalse(), "with an existing file we shouldn't do the step")
}

func TestDiskCreate_Do(t *testing.T) {
	g.RegisterTestingT(t)

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	diskSvc := mock.NewMockDiskService(mockCtrl)
	ctx := context.Background()
	fs := afero.NewMemMapFs()

	input := runtime.DiskCreateStepInput{
		Path:           "/tmp/test.img",
		VolumeName:     "data",
		Size:           "8Mb",
		DiskType:       ports.DiskTypeFat32,
		Content:        []ports.DiskFile{},
		AlwaysRecreate: false,
	}

	diskSvc.EXPECT().Create(ctx, ports.DiskCreateInput{
		Path:       input.Path,
		Size:       input.Size,
		VolumeName: input.VolumeName,
		Type:       input.DiskType,
		Files:      input.Content,
	}).Return(nil).Times(1)

	step := runtime.NewDiskCreateStep(&input, diskSvc, fs)
	childSteps, err := step.Do(ctx)
	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(childSteps).To(g.HaveLen(0))
}

func TestDiskCreate_Do_InvalidSize(t *testing.T) {
	g.RegisterTestingT(t)

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	diskSvc := mock.NewMockDiskService(mockCtrl)
	ctx := context.Background()
	fs := afero.NewMemMapFs()

	input := runtime.DiskCreateStepInput{
		Path:           "/tmp/test.img",
		VolumeName:     "data",
		Size:           "8zz",
		DiskType:       ports.DiskTypeFat32,
		Content:        []ports.DiskFile{},
		AlwaysRecreate: false,
	}

	diskSvc.EXPECT().Create(ctx, ports.DiskCreateInput{
		Path:       input.Path,
		Size:       input.Size,
		VolumeName: input.VolumeName,
		Type:       input.DiskType,
		Files:      input.Content,
	}).Return(errors.New("couldn't convert size")).Times(1)

	step := runtime.NewDiskCreateStep(&input, diskSvc, fs)
	_, err := step.Do(ctx)
	g.Expect(err).To(g.HaveOccurred())
}
