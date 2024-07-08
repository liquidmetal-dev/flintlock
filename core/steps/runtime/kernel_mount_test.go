package runtime_test

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	internalerr "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/core/steps/runtime"
	"github.com/liquidmetal-dev/flintlock/infrastructure/mock"
	g "github.com/onsi/gomega"
)

func testVMWithKernel() *models.MicroVM {
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
			VCPU:       1,
			MemoryInMb: 512,
		},
	}
}

func testKernelMount(source string) models.Mount {
	return models.Mount{
		Type:   "mounttype",
		Source: source,
	}
}

func testKernelMountSpec(vm *models.MicroVM) *ports.ImageMountSpec {
	return &ports.ImageMountSpec{
		ImageName:    string(vm.Spec.Kernel.Image),
		Owner:        vm.ID.String(),
		OwnerUsageID: "kernel",
		Use:          models.ImageUseKernel,
	}
}

func TestKernelMount(t *testing.T) {
	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	imageService := mock.NewMockImageService(mockCtrl)
	ctx := context.Background()
	vm := testVMWithKernel()

	// The Do function sets the KernelMount to the first element of
	// the returned slice from PullAndMount. The status should be
	// the first one and not the second one.
	expectedMount := testKernelMount("firstsouArce")
	notExpectedMount := testKernelMount("secondsource")

	step := runtime.NewKernelMount(vm, imageService)

	imageService.
		EXPECT().
		PullAndMount(ctx, testKernelMountSpec(vm)).
		Return([]models.Mount{expectedMount, notExpectedMount}, nil)

	shouldDo, shouldErr := step.ShouldDo(ctx)
	subSteps, doErr := step.Do(ctx)

	g.Expect(shouldDo).To(g.BeTrue())
	g.Expect(shouldErr).To(g.BeNil())
	g.Expect(subSteps).To(g.BeEmpty())
	g.Expect(doErr).To(g.BeNil())

	g.Expect(vm.Status.KernelMount).To(
		g.BeEquivalentTo(&expectedMount),
	)

	verifyErr := step.Verify(ctx)
	g.Expect(verifyErr).To(g.BeNil())
}

func TestKernelMount_noKernel(t *testing.T) {
	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	imageService := mock.NewMockImageService(mockCtrl)
	ctx := context.Background()
	vm := testVMWithKernel()

	vm.Spec.Kernel = models.Kernel{}

	step := runtime.NewKernelMount(vm, imageService)

	shouldDo, shouldErr := step.ShouldDo(ctx)
	subSteps, doErr := step.Do(ctx)

	g.Expect(shouldDo).To(g.BeTrue())
	g.Expect(shouldErr).To(g.BeNil())

	g.Expect(doErr).To(g.MatchError(internalerr.ErrKernelImageRequired))
	g.Expect(subSteps).To(g.BeEmpty())

	verifyErr := step.Verify(ctx)
	g.Expect(verifyErr).To(g.BeNil())
}

func TestKernelMount_statusAlreadySet(t *testing.T) {
	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	imageService := mock.NewMockImageService(mockCtrl)
	ctx := context.Background()
	vm := testVMWithKernel()
	expectedMount := testKernelMount("mysource")
	vm.Status.KernelMount = &expectedMount
	mountSpec := testKernelMountSpec(vm)

	step := runtime.NewKernelMount(vm, imageService)

	// Already mounted.
	imageService.
		EXPECT().
		IsMounted(ctx, mountSpec).
		Return(true, nil)

	shouldDo, shouldErr := step.ShouldDo(ctx)

	g.Expect(shouldDo).To(g.BeFalse())
	g.Expect(shouldErr).To(g.BeNil())

	// Not mounted.
	imageService.
		EXPECT().
		IsMounted(ctx, mountSpec).
		Return(false, nil)

	shouldDo, shouldErr = step.ShouldDo(ctx)

	g.Expect(shouldDo).To(g.BeTrue())
	g.Expect(shouldErr).To(g.BeNil())

	// Service error.
	imageService.
		EXPECT().
		IsMounted(ctx, mountSpec).
		Return(false, errors.New("nope"))

	shouldDo, shouldErr = step.ShouldDo(ctx)

	g.Expect(shouldDo).To(g.BeFalse())
	g.Expect(shouldErr).ToNot(g.BeNil())

	verifyErr := step.Verify(ctx)
	g.Expect(verifyErr).To(g.BeNil())
}

func TestKernelMount_vmNotSet(t *testing.T) {
	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	imageService := mock.NewMockImageService(mockCtrl)
	ctx := context.Background()
	var vm *models.MicroVM

	step := runtime.NewKernelMount(vm, imageService)

	shouldDo, shouldErr := step.ShouldDo(ctx)
	subSteps, doErr := step.Do(ctx)

	g.Expect(shouldDo).To(g.BeFalse())
	g.Expect(shouldErr).To(g.MatchError(internalerr.ErrSpecRequired))
	g.Expect(subSteps).To(g.BeEmpty())
	g.Expect(doErr).To(g.MatchError(internalerr.ErrSpecRequired))

	verifyErr := step.Verify(ctx)
	g.Expect(verifyErr).To(g.BeNil())
}

func TestKernelMount_pullAndMountError(t *testing.T) {
	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	imageService := mock.NewMockImageService(mockCtrl)
	ctx := context.Background()
	vm := testVMWithKernel()

	step := runtime.NewKernelMount(vm, imageService)

	imageService.
		EXPECT().
		PullAndMount(ctx, testKernelMountSpec(vm)).
		Return([]models.Mount{}, errors.New("pull error"))

	subSteps, doErr := step.Do(ctx)

	g.Expect(subSteps).To(g.BeEmpty())
	g.Expect(doErr).ToNot(g.BeNil())

	verifyErr := step.Verify(ctx)
	g.Expect(verifyErr).To(g.BeNil())
}

func TestKernelMount_emptyResponse(t *testing.T) {
	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	imageService := mock.NewMockImageService(mockCtrl)
	ctx := context.Background()
	vm := testVMWithKernel()

	step := runtime.NewKernelMount(vm, imageService)

	imageService.
		EXPECT().
		PullAndMount(ctx, testKernelMountSpec(vm)).
		Return([]models.Mount{}, nil)

	subSteps, doErr := step.Do(ctx)

	g.Expect(subSteps).To(g.BeEmpty())
	g.Expect(doErr).To(g.MatchError(internalerr.ErrNoMount))

	verifyErr := step.Verify(ctx)
	g.Expect(verifyErr).To(g.BeNil())
}
