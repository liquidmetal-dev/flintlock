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

const bootTimeInSeconds = 1

func testVMToStart() *models.MicroVM {
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

func TestNewStartStep(t *testing.T) {
	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	microVMService := mock.NewMockMicroVMService(mockCtrl)
	ctx := context.Background()
	vm := testVMToStart()

	step := microvm.NewStartStep(vm, microVMService, bootTimeInSeconds)

	microVMService.
		EXPECT().
		State(ctx, vm.ID.String()).
		Return(ports.MicroVMStateConfigured, nil)

	microVMService.
		EXPECT().
		State(ctx, vm.ID.String()).
		Return(ports.MicroVMStateRunning, nil)

	microVMService.
		EXPECT().
		Start(ctx, vm).
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

func TestNewStartStep_StateCheck(t *testing.T) {
	type stateCheck struct {
		State       ports.MicroVMState
		ExpectToRun bool
	}

	stateTestCases := []stateCheck{
		{State: ports.MicroVMStatePending, ExpectToRun: true},
		{State: ports.MicroVMStateConfigured, ExpectToRun: true},
		{State: ports.MicroVMStateRunning, ExpectToRun: false},
		{State: ports.MicroVMStateUnknown, ExpectToRun: true},
	}

	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	microVMService := mock.NewMockMicroVMService(mockCtrl)
	ctx := context.Background()
	vm := testVMToStart()

	step := microvm.NewStartStep(vm, microVMService, bootTimeInSeconds)

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

	step := microvm.NewStartStep(vm, microVMService, bootTimeInSeconds)

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

	step := microvm.NewStartStep(vm, microVMService, bootTimeInSeconds)

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

	step := microvm.NewStartStep(vm, microVMService, bootTimeInSeconds)

	microVMService.
		EXPECT().
		Start(ctx, vm).
		Return(errors.New("nope"))

	subSteps, err := step.Do(ctx)

	g.Expect(subSteps).To(g.BeEmpty())
	g.Expect(err).ToNot(g.BeNil())
}

func TestNewStartStep_unableToBoot(t *testing.T) {
	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	microVMService := mock.NewMockMicroVMService(mockCtrl)
	vm := testVMToStart()
	ctx := context.Background()

	step := microvm.NewStartStep(vm, microVMService, bootTimeInSeconds)

	microVMService.
		EXPECT().
		Start(ctx, vm).
		Return(nil)

	microVMService.
		EXPECT().
		State(ctx, vm.ID.String()).
		Return(ports.MicroVMStateUnknown, nil)

	subSteps, err := step.Do(ctx)
	g.Expect(err).To(g.BeNil())
	g.Expect(subSteps).To(g.BeEmpty())

	verifyErr := step.Verify(ctx)
	g.Expect(verifyErr).To(g.MatchError(internalerr.ErrUnableToBoot))

	microVMService.
		EXPECT().
		State(ctx, vm.ID.String()).
		Return(ports.MicroVMStateUnknown, errors.New("nope"))

	verifyErr = step.Verify(ctx)

	g.Expect(verifyErr).ToNot(g.BeNil())
	g.Expect(verifyErr).ToNot(g.MatchError(internalerr.ErrUnableToBoot))
}
