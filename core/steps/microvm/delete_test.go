package microvm_test

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	internalerr "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/core/steps/microvm"
	"github.com/liquidmetal-dev/flintlock/infrastructure/mock"
	g "github.com/onsi/gomega"
)

func testVMToDelete() *models.MicroVM {
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
			DeletedAt:  1,
		},
	}
}

func TestNewDeleteStep(t *testing.T) {
	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	microVMService := mock.NewMockMicroVMService(mockCtrl)
	ctx := context.Background()
	vm := testVMToDelete()

	step := microvm.NewDeleteStep(vm, microVMService)

	microVMService.
		EXPECT().
		State(ctx, vm.ID.String()).
		Return(ports.MicroVMStateRunning, nil)

	microVMService.
		EXPECT().
		Delete(ctx, vm.ID.String()).
		Return(nil)

	shouldDo, shouldErr := step.ShouldDo(ctx)
	subSteps, doErr := step.Do(ctx)
	verifyErr := step.Verify(ctx)

	g.Expect(shouldDo).To(g.BeTrue())
	g.Expect(shouldErr).To(g.BeNil())
	g.Expect(subSteps).To(g.BeEmpty())
	g.Expect(doErr).To(g.BeNil())
	g.Expect(verifyErr).To(g.BeNil())
}

func TestNewDeleteStep_StateCheck(t *testing.T) {
	type stateCheck struct {
		State       ports.MicroVMState
		ExpectToRun bool
	}

	stateTestCases := []stateCheck{
		{State: ports.MicroVMStatePending, ExpectToRun: false},
		{State: ports.MicroVMStateConfigured, ExpectToRun: true},
		{State: ports.MicroVMStateRunning, ExpectToRun: true},
		{State: ports.MicroVMStateUnknown, ExpectToRun: false},
	}

	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	microVMService := mock.NewMockMicroVMService(mockCtrl)
	ctx := context.Background()
	vm := testVMToDelete()

	step := microvm.NewDeleteStep(vm, microVMService)

	for _, testCase := range stateTestCases {
		microVMService.
			EXPECT().
			State(ctx, vm.ID.String()).
			Return(testCase.State, nil)

		shouldDo, shouldErr := step.ShouldDo(ctx)
		verifyErr := step.Verify(ctx)

		g.Expect(shouldDo).To(g.Equal(testCase.ExpectToRun))
		g.Expect(shouldErr).To(g.BeNil())
		g.Expect(verifyErr).To(g.BeNil())
	}
}

func TestNewDeleteStep_StateCheckError(t *testing.T) {
	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	microVMService := mock.NewMockMicroVMService(mockCtrl)
	ctx := context.Background()
	vm := testVMToDelete()

	step := microvm.NewDeleteStep(vm, microVMService)

	microVMService.
		EXPECT().
		State(ctx, vm.ID.String()).
		Return(ports.MicroVMStateUnknown, errors.New("i have no idea"))

	shouldDo, shouldErr := step.ShouldDo(ctx)
	verifyErr := step.Verify(ctx)

	g.Expect(shouldDo).To(g.BeFalse())
	g.Expect(shouldErr).ToNot(g.BeNil())
	g.Expect(verifyErr).To(g.BeNil())
}

func TestNewDeleteStep_VMIsNotDefined(t *testing.T) {
	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	var vm *models.MicroVM

	microVMService := mock.NewMockMicroVMService(mockCtrl)
	ctx := context.Background()

	step := microvm.NewDeleteStep(vm, microVMService)

	subSteps, err := step.Do(ctx)
	verifyErr := step.Verify(ctx)

	g.Expect(subSteps).To(g.BeEmpty())
	g.Expect(err).To(g.MatchError(internalerr.ErrSpecRequired))
	g.Expect(verifyErr).To(g.BeNil())
}

func TestNewDeleteStep_ServiceDeleteError(t *testing.T) {
	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	microVMService := mock.NewMockMicroVMService(mockCtrl)
	vm := testVMToDelete()
	ctx := context.Background()

	step := microvm.NewDeleteStep(vm, microVMService)

	microVMService.
		EXPECT().
		Delete(ctx, vm.ID.String()).
		Return(errors.New("ensuring state dir: ...."))

	subSteps, err := step.Do(ctx)
	verifyErr := step.Verify(ctx)

	g.Expect(subSteps).To(g.BeEmpty())
	g.Expect(err).ToNot(g.BeNil())
	g.Expect(verifyErr).To(g.BeNil())
}
