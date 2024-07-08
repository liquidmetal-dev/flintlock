package runtime_test

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	internalerrors "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/steps/runtime"
	"github.com/liquidmetal-dev/flintlock/infrastructure/mock"
	g "github.com/onsi/gomega"
)

func testVM() *models.MicroVM {
	vmid, _ := models.NewVMID("vm", "ns", "uid")
	return &models.MicroVM{
		ID:      *vmid,
		Version: 1,
		Spec: models.MicroVMSpec{
			Kernel: models.Kernel{
				Image:            "image:tag",
				Filename:         "/vmlinuz",
				AddNetworkConfig: true,
			},
			Initrd: &models.Initrd{
				Image:    "image:tag",
				Filename: "/initrd",
			},
			VCPU:       1,
			MemoryInMb: 512,
		},
	}
}

func TestNewRepoRelease(t *testing.T) {
	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	microVMRepoService := mock.NewMockMicroVMRepository(mockCtrl)
	ctx := context.Background()
	vm := testVM()

	step := runtime.NewRepoRelease(vm, microVMRepoService)

	microVMRepoService.
		EXPECT().
		Exists(ctx, vm.ID).
		Return(true, nil)

	microVMRepoService.
		EXPECT().
		ReleaseLease(ctx, vm).
		Return(nil)

	shouldDo, shouldErr := step.ShouldDo(ctx)
	subSteps, doErr := step.Do(ctx)

	g.Expect(shouldDo).To(g.BeTrue())
	g.Expect(shouldErr).To(g.BeNil())
	g.Expect(subSteps).To(g.BeEmpty())
	g.Expect(doErr).To(g.BeNil())

	verifyErr := step.Verify(ctx)
	g.Expect(verifyErr).To(g.BeNil())
}

func TestNewRepoRelease_doesNotExist(t *testing.T) {
	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	microVMRepoService := mock.NewMockMicroVMRepository(mockCtrl)
	ctx := context.Background()
	vm := testVM()

	step := runtime.NewRepoRelease(vm, microVMRepoService)

	microVMRepoService.
		EXPECT().
		Exists(ctx, vm.ID).
		Return(false, nil)

	shouldDo, shouldErr := step.ShouldDo(ctx)

	g.Expect(shouldDo).To(g.BeFalse())
	g.Expect(shouldErr).To(g.BeNil())

	verifyErr := step.Verify(ctx)
	g.Expect(verifyErr).To(g.BeNil())
}

func TestNewRepoRelease_VMIsNotDefined(t *testing.T) {
	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	microVMRepoService := mock.NewMockMicroVMRepository(mockCtrl)
	ctx := context.Background()

	var vm *models.MicroVM

	step := runtime.NewRepoRelease(vm, microVMRepoService)

	shouldDo, shouldErr := step.ShouldDo(ctx)
	subSteps, doErr := step.Do(ctx)

	g.Expect(shouldDo).To(g.BeFalse())
	g.Expect(shouldErr).To(g.MatchError(internalerrors.ErrSpecRequired))
	g.Expect(subSteps).To(g.BeEmpty())
	g.Expect(doErr).To(g.MatchError(internalerrors.ErrSpecRequired))

	verifyErr := step.Verify(ctx)
	g.Expect(verifyErr).To(g.BeNil())
}

func TestNewRepoRelease_existsCheckFails(t *testing.T) {
	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	microVMRepoService := mock.NewMockMicroVMRepository(mockCtrl)
	ctx := context.Background()
	vm := testVM()

	step := runtime.NewRepoRelease(vm, microVMRepoService)

	microVMRepoService.
		EXPECT().
		Exists(ctx, vm.ID).
		Return(false, errors.New("exists check failed"))

	shouldDo, shouldErr := step.ShouldDo(ctx)

	g.Expect(shouldDo).To(g.BeFalse())
	g.Expect(shouldErr).ToNot(g.BeNil())

	verifyErr := step.Verify(ctx)
	g.Expect(verifyErr).To(g.BeNil())
}

func TestNewRepoRelease_repoServiceError(t *testing.T) {
	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	microVMRepoService := mock.NewMockMicroVMRepository(mockCtrl)
	ctx := context.Background()
	vm := testVM()

	step := runtime.NewRepoRelease(vm, microVMRepoService)

	microVMRepoService.
		EXPECT().
		Exists(ctx, vm.ID).
		Return(true, nil)

	microVMRepoService.
		EXPECT().
		ReleaseLease(ctx, vm).
		Return(errors.New("something went wrong"))

	shouldDo, shouldErr := step.ShouldDo(ctx)
	subSteps, doErr := step.Do(ctx)

	g.Expect(shouldDo).To(g.BeTrue())
	g.Expect(shouldErr).To(g.BeNil())
	g.Expect(subSteps).To(g.BeEmpty())
	g.Expect(doErr).ToNot(g.BeNil())

	verifyErr := step.Verify(ctx)
	g.Expect(verifyErr).To(g.BeNil())
}
