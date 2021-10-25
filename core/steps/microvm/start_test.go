package microvm_test

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	g "github.com/onsi/gomega"
	internalerr "github.com/weaveworks/flintlock/core/errors"
	"github.com/weaveworks/flintlock/core/models"
	"github.com/weaveworks/flintlock/core/ports"
	"github.com/weaveworks/flintlock/core/steps/microvm"
	"github.com/weaveworks/flintlock/infrastructure/mock"
)

func testVMToStart() *models.MicroVM {
	vmid, _ := models.NewVMID("vm", "ns")
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

func TestNewStartStep(t *testing.T) {
	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	microVMService := mock.NewMockMicroVMService(mockCtrl)
	ctx := context.Background()
	vm := testVMToStart()

	step := microvm.NewStartStep(vm, microVMService)

	microVMService.
		EXPECT().
		State(ctx, vm.ID.String()).
		Return(ports.MicroVMStateConfigured, nil)

	microVMService.
		EXPECT().
		Start(ctx, vm.ID.String()).
		Return(nil)

	shouldDo, shouldErr := step.ShouldDo(ctx)
	subSteps, doErr := step.Do(ctx)

	g.Expect(shouldDo).To(g.BeTrue())
	g.Expect(shouldErr).To(g.BeNil())
	g.Expect(subSteps).To(g.BeEmpty())
	g.Expect(doErr).To(g.BeNil())
}

func TestNewStartStep_StateCheck(t *testing.T) {
	type stateCheck struct {
		State       ports.MicroVMState
		ExpectToRun bool
	}

	stateTestCases := []stateCheck{
		{State: ports.MicroVMStatePending, ExpectToRun: true},
		{State: ports.MicroVMStateConfigured, ExpectToRun: true},
		{State: ports.MicroVMStateRunning, ExpectToRun: false},
		{State: ports.MicroVMStatePaused, ExpectToRun: true},
		{State: ports.MicroVMStateUnknown, ExpectToRun: true},
	}

	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	microVMService := mock.NewMockMicroVMService(mockCtrl)
	ctx := context.Background()
	vm := testVMToStart()

	step := microvm.NewStartStep(vm, microVMService)

	for _, testCase := range stateTestCases {
		microVMService.
			EXPECT().
			State(ctx, vm.ID.String()).
			Return(testCase.State, nil)

		shouldDo, shouldErr := step.ShouldDo(ctx)

		g.Expect(shouldDo).To(g.Equal(testCase.ExpectToRun))
		g.Expect(shouldErr).To(g.BeNil())
	}

}

func TestNewStartStep_StateCheckError(t *testing.T) {
	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	microVMService := mock.NewMockMicroVMService(mockCtrl)
	ctx := context.Background()
	vm := testVMToStart()

	step := microvm.NewStartStep(vm, microVMService)

	microVMService.
		EXPECT().
		State(ctx, vm.ID.String()).
		Return(ports.MicroVMStateUnknown, errors.New("i have no idea"))

	shouldDo, shouldErr := step.ShouldDo(ctx)

	g.Expect(shouldDo).To(g.BeFalse())
	g.Expect(shouldErr).ToNot(g.BeNil())
}

func TestNewStartStep_VMIsNotDefined(t *testing.T) {
	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	var vm *models.MicroVM

	microVMService := mock.NewMockMicroVMService(mockCtrl)
	ctx := context.Background()

	step := microvm.NewStartStep(vm, microVMService)

	subSteps, err := step.Do(ctx)

	g.Expect(subSteps).To(g.BeEmpty())
	g.Expect(err).To(g.MatchError(internalerr.ErrSpecRequired))
}

func TestNewStartStep_ServiceStartError(t *testing.T) {
	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	microVMService := mock.NewMockMicroVMService(mockCtrl)
	vm := testVMToStart()
	ctx := context.Background()

	step := microvm.NewStartStep(vm, microVMService)

	microVMService.
		EXPECT().
		Start(ctx, vm.ID.String()).
		Return(errors.New("nope"))

	subSteps, err := step.Do(ctx)

	g.Expect(subSteps).To(g.BeEmpty())
	g.Expect(err).ToNot(g.BeNil())
}
