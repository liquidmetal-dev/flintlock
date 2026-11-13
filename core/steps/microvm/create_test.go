package microvm_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	g "github.com/onsi/gomega"

	internalerr "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/core/steps/microvm"
	"github.com/liquidmetal-dev/flintlock/infrastructure/mock"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
)

func testVMToCreate() *models.MicroVM {
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

func TestNewCreateStep(t *testing.T) {
	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	microVMService := mock.NewMockMicroVMService(mockCtrl)
	ctx := context.Background()
	vm := testVMToCreate()

	step := microvm.NewCreateStep(vm, microVMService, testWait)

	microVMService.
		EXPECT().
		State(ctx, vm.ID.String()).
		Return(ports.MicroVMStatePending, nil)

	microVMService.
		EXPECT().
		Create(ctx, vm).
		Return(nil)

	microVMService.
		EXPECT().
		State(ctx, vm.ID.String()).
		Return(ports.MicroVMStateRunning, nil)

	shouldDo, shouldErr := step.ShouldDo(ctx)
	subSteps, doErr := step.Do(ctx)
	verifyErr := step.Verify(ctx)

	g.Expect(shouldDo).To(g.BeTrue())
	g.Expect(shouldErr).To(g.BeNil())
	g.Expect(subSteps).To(g.BeEmpty())
	g.Expect(doErr).To(g.BeNil())
	g.Expect(verifyErr).To(g.BeNil())
}

func TestNewCreateStep_StateCheck(t *testing.T) {
	type stateCheck struct {
		State       ports.MicroVMState
		ExpectToRun bool
	}

	stateTestCases := []stateCheck{
		{State: ports.MicroVMStatePending, ExpectToRun: true},
		{State: ports.MicroVMStateConfigured, ExpectToRun: false},
		{State: ports.MicroVMStateRunning, ExpectToRun: false},
		{State: ports.MicroVMStatePaused, ExpectToRun: false},
		{State: ports.MicroVMStateUnknown, ExpectToRun: false},
	}

	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	microVMService := mock.NewMockMicroVMService(mockCtrl)
	ctx := context.Background()
	vm := testVMToCreate()

	step := microvm.NewCreateStep(vm, microVMService, testWait)

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

func TestNewCreateStep_StateCheckError(t *testing.T) {
	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	microVMService := mock.NewMockMicroVMService(mockCtrl)
	ctx := context.Background()
	vm := testVMToCreate()

	step := microvm.NewCreateStep(vm, microVMService, testWait)

	microVMService.
		EXPECT().
		State(ctx, vm.ID.String()).
		Return(ports.MicroVMStateUnknown, errors.New("i have no idea"))

	shouldDo, shouldErr := step.ShouldDo(ctx)

	g.Expect(shouldDo).To(g.BeFalse())
	g.Expect(shouldErr).ToNot(g.BeNil())
}

func TestNewCreateStep_VMIsNotDefined(t *testing.T) {
	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	var vm *models.MicroVM

	microVMService := mock.NewMockMicroVMService(mockCtrl)
	ctx := context.Background()

	step := microvm.NewCreateStep(vm, microVMService, testWait)

	subSteps, err := step.Do(ctx)
	verifyErr := step.Verify(ctx)

	g.Expect(subSteps).To(g.BeEmpty())
	g.Expect(err).To(g.MatchError(internalerr.ErrSpecRequired))
	g.Expect(verifyErr).To(g.MatchError(internalerr.ErrSpecRequired))
}

func TestNewCreateStep_ServiceCreateError(t *testing.T) {
	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	microVMService := mock.NewMockMicroVMService(mockCtrl)
	vm := testVMToCreate()
	ctx := context.Background()

	step := microvm.NewCreateStep(vm, microVMService, testWait)

	microVMService.
		EXPECT().
		Create(ctx, vm).
		Return(errors.New("ensuring state dir: ...."))

	subSteps, err := step.Do(ctx)

	g.Expect(subSteps).To(g.BeEmpty())
	g.Expect(err).ToNot(g.BeNil())
}

func TestCreateStep_Verify_SetsVMMRunningTrue(t *testing.T) {
	g.RegisterTestingT(t)
	svc := mock.NewMockMicroVMService(gomock.NewController(t))
	ctx := context.Background()
	vm := testVMToCreate()

	gomock.InOrder(
		svc.EXPECT().State(ctx, vm.ID.String()).Return(ports.MicroVMStateConfigured, nil),
		svc.EXPECT().State(ctx, vm.ID.String()).Return(ports.MicroVMStateRunning, nil),
	)

	step := microvm.NewCreateStep(vm, svc, testWait)

	g.Expect(step.Verify(ctx)).To(g.Succeed())

	cond, ok := vm.Status.Conditions.Get(models.ConditionVMMRunning)
	g.Expect(ok).To(g.BeTrue())
	g.Expect(cond.Status).To(g.Equal(models.ConditionTrue))
	g.Expect(cond.Reason).To(g.Equal(models.ReasonRunning))
	g.Expect(cond.LastTransition).To(g.Equal(testNow))
}

func TestCreateStep_Verify_SetsVMMRunningFalseOnTimeout(t *testing.T) {
	g.RegisterTestingT(t)
	svc := mock.NewMockMicroVMService(gomock.NewController(t))
	ctx := context.Background()
	vm := testVMToCreate()

	svc.EXPECT().State(ctx, vm.ID.String()).Return(ports.MicroVMStatePending, nil).AnyTimes()

	step := microvm.NewCreateStep(vm, svc, testWait)

	g.Expect(step.Verify(ctx)).To(g.MatchError(internalerr.ErrUnableToBoot))

	cond, ok := vm.Status.Conditions.Get(models.ConditionVMMRunning)
	g.Expect(ok).To(g.BeTrue())
	g.Expect(cond.Status).To(g.Equal(models.ConditionFalse))
	g.Expect(cond.Reason).To(g.Equal(models.ReasonStartTimeout))
	g.Expect(cond.Message).To(g.ContainSubstring("pending"))
}

func TestCreateStep_Verify_NoFixedWait(t *testing.T) {
	g.RegisterTestingT(t)
	svc := mock.NewMockMicroVMService(gomock.NewController(t))
	ctx := context.Background()
	vm := testVMToCreate()
	vm.Spec.AllowGuestAgent = true

	svc.EXPECT().State(ctx, vm.ID.String()).Return(ports.MicroVMStateRunning, nil)

	step := microvm.NewCreateStep(vm, svc, microvm.WaitConfig{
		Timeout:  30 * time.Second,
		Interval: defaults.ReadyPollInterval,
		Clock:    testClock,
	})

	start := time.Now()
	g.Expect(step.Verify(ctx)).To(g.Succeed())
	g.Expect(time.Since(start)).To(g.BeNumerically("<", 100*time.Millisecond))
}

func TestCreateStep_Do_ResetsGuestAgentReady(t *testing.T) {
	g.RegisterTestingT(t)
	svc := mock.NewMockMicroVMService(gomock.NewController(t))
	ctx := context.Background()

	vm := testVMToCreate()
	vm.Spec.AllowGuestAgent = true
	vm.Status.Conditions.Set(models.Condition{
		Type:   models.ConditionGuestAgentReady,
		Status: models.ConditionTrue,
	}, testNow.Add(-time.Hour))

	svc.EXPECT().Create(ctx, vm).Return(nil)

	_, err := microvm.NewCreateStep(vm, svc, testWait).Do(ctx)
	g.Expect(err).NotTo(g.HaveOccurred())

	cond, ok := vm.Status.Conditions.Get(models.ConditionGuestAgentReady)
	g.Expect(ok).To(g.BeTrue())
	g.Expect(cond.Status).To(g.Equal(models.ConditionUnknown))
	g.Expect(cond.Reason).To(g.Equal(models.ReasonVMMStarting))
	g.Expect(cond.LastTransition).To(g.Equal(testNow))
}

func TestCreateStep_Do_NoGuestAgentNoCondition(t *testing.T) {
	g.RegisterTestingT(t)
	svc := mock.NewMockMicroVMService(gomock.NewController(t))
	ctx := context.Background()
	vm := testVMToCreate()

	svc.EXPECT().Create(ctx, vm).Return(nil)

	_, err := microvm.NewCreateStep(vm, svc, testWait).Do(ctx)
	g.Expect(err).NotTo(g.HaveOccurred())

	_, ok := vm.Status.Conditions.Get(models.ConditionGuestAgentReady)
	g.Expect(ok).To(g.BeFalse())
}
