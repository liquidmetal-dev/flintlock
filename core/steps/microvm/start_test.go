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

var (
	testNow  = time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	testWait = microvm.WaitConfig{
		Timeout:  20 * time.Millisecond,
		Interval: time.Millisecond,
		Clock:    testClock,
	}
)

func testClock() time.Time {
	return testNow
}

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

	step := microvm.NewStartStep(vm, microVMService, testWait)

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
		{State: ports.MicroVMStatePaused, ExpectToRun: false},
		{State: ports.MicroVMStateUnknown, ExpectToRun: true},
	}

	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	microVMService := mock.NewMockMicroVMService(mockCtrl)
	ctx := context.Background()
	vm := testVMToStart()

	step := microvm.NewStartStep(vm, microVMService, testWait)

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

	step := microvm.NewStartStep(vm, microVMService, testWait)

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

	step := microvm.NewStartStep(vm, microVMService, testWait)

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

	step := microvm.NewStartStep(vm, microVMService, testWait)

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

	step := microvm.NewStartStep(vm, microVMService, testWait)

	microVMService.
		EXPECT().
		Start(ctx, vm).
		Return(nil)

	microVMService.
		EXPECT().
		State(ctx, vm.ID.String()).
		Return(ports.MicroVMStateUnknown, nil).
		AnyTimes()

	subSteps, err := step.Do(ctx)
	g.Expect(err).To(g.BeNil())
	g.Expect(subSteps).To(g.BeEmpty())

	verifyErr := step.Verify(ctx)
	g.Expect(verifyErr).To(g.MatchError(internalerr.ErrUnableToBoot))
}

func TestStartStep_Verify_StateError(t *testing.T) {
	g.RegisterTestingT(t)
	svc := mock.NewMockMicroVMService(gomock.NewController(t))
	vm := testVMToStart()
	ctx := context.Background()

	svc.EXPECT().State(ctx, vm.ID.String()).Return(ports.MicroVMStateUnknown, errors.New("nope"))

	verifyErr := microvm.NewStartStep(vm, svc, testWait).Verify(ctx)

	g.Expect(verifyErr).ToNot(g.BeNil())
	g.Expect(verifyErr).ToNot(g.MatchError(internalerr.ErrUnableToBoot))
}

func TestStartStep_Verify_SetsVMMRunningTrue(t *testing.T) {
	g.RegisterTestingT(t)
	svc := mock.NewMockMicroVMService(gomock.NewController(t))
	vm := testVMToStart()
	ctx := context.Background()

	svc.EXPECT().State(ctx, vm.ID.String()).Return(ports.MicroVMStateRunning, nil)

	g.Expect(microvm.NewStartStep(vm, svc, testWait).Verify(ctx)).To(g.Succeed())

	cond, ok := vm.Status.Conditions.Get(models.ConditionVMMRunning)
	g.Expect(ok).To(g.BeTrue())
	g.Expect(cond.Status).To(g.Equal(models.ConditionTrue))
	g.Expect(cond.Reason).To(g.Equal(models.ReasonRunning))
	g.Expect(cond.LastTransition).To(g.Equal(testNow))
}

func TestStartStep_Verify_SetsVMMRunningFalseOnTimeout(t *testing.T) {
	g.RegisterTestingT(t)
	svc := mock.NewMockMicroVMService(gomock.NewController(t))
	vm := testVMToStart()
	ctx := context.Background()

	svc.EXPECT().State(ctx, vm.ID.String()).Return(ports.MicroVMStatePending, nil).AnyTimes()

	err := microvm.NewStartStep(vm, svc, testWait).Verify(ctx)
	g.Expect(err).To(g.MatchError(internalerr.ErrUnableToBoot))

	cond, ok := vm.Status.Conditions.Get(models.ConditionVMMRunning)
	g.Expect(ok).To(g.BeTrue())
	g.Expect(cond.Status).To(g.Equal(models.ConditionFalse))
	g.Expect(cond.Reason).To(g.Equal(models.ReasonStartTimeout))
	g.Expect(cond.Message).To(g.ContainSubstring("pending"))
}

func TestStartStep_Verify_NoFixedWait(t *testing.T) {
	g.RegisterTestingT(t)
	svc := mock.NewMockMicroVMService(gomock.NewController(t))
	vm := testVMToStart()
	vm.Spec.AllowGuestAgent = true
	ctx := context.Background()

	svc.EXPECT().State(ctx, vm.ID.String()).Return(ports.MicroVMStateRunning, nil)

	step := microvm.NewStartStep(vm, svc, microvm.WaitConfig{
		Timeout:  30 * time.Second,
		Interval: defaults.ReadyPollInterval,
		Clock:    testClock,
	})

	start := time.Now()
	g.Expect(step.Verify(ctx)).To(g.Succeed())
	g.Expect(time.Since(start)).To(g.BeNumerically("<", 100*time.Millisecond))
}

func TestStartStep_Do_ResetsGuestAgentReady(t *testing.T) {
	g.RegisterTestingT(t)
	svc := mock.NewMockMicroVMService(gomock.NewController(t))
	ctx := context.Background()

	vm := testVMToStart()
	vm.Spec.AllowGuestAgent = true
	vm.Status.Conditions.Set(models.Condition{
		Type:   models.ConditionGuestAgentReady,
		Status: models.ConditionTrue,
	}, testNow.Add(-time.Hour))

	svc.EXPECT().Start(ctx, vm).Return(nil)

	_, err := microvm.NewStartStep(vm, svc, testWait).Do(ctx)
	g.Expect(err).NotTo(g.HaveOccurred())

	cond, ok := vm.Status.Conditions.Get(models.ConditionGuestAgentReady)
	g.Expect(ok).To(g.BeTrue())
	g.Expect(cond.Status).To(g.Equal(models.ConditionUnknown))
	g.Expect(cond.Reason).To(g.Equal(models.ReasonVMMStarting))
	g.Expect(cond.LastTransition).To(g.Equal(testNow))
}

func TestStartStep_Do_NoGuestAgentNoCondition(t *testing.T) {
	g.RegisterTestingT(t)
	svc := mock.NewMockMicroVMService(gomock.NewController(t))
	ctx := context.Background()
	vm := testVMToStart()

	svc.EXPECT().Start(ctx, vm).Return(nil)

	_, err := microvm.NewStartStep(vm, svc, testWait).Do(ctx)
	g.Expect(err).NotTo(g.HaveOccurred())

	_, ok := vm.Status.Conditions.Get(models.ConditionGuestAgentReady)
	g.Expect(ok).To(g.BeFalse())
}

func TestStartStep_Do_ResetsConditionsWhenStartFails(t *testing.T) {
	g.RegisterTestingT(t)
	svc := mock.NewMockMicroVMService(gomock.NewController(t))
	ctx := context.Background()

	vm := testVMToStart()
	vm.Spec.AllowGuestAgent = true
	vm.Status.Conditions.Set(models.Condition{Type: models.ConditionVMMRunning, Status: models.ConditionTrue}, testNow)
	vm.Status.Conditions.Set(models.Condition{Type: models.ConditionGuestAgentReady, Status: models.ConditionTrue}, testNow)

	svc.EXPECT().Start(ctx, vm).Return(errors.New("start failed"))

	_, err := microvm.NewStartStep(vm, svc, testWait).Do(ctx)
	g.Expect(err).To(g.HaveOccurred())

	ready, _ := vm.Status.Conditions.Get(models.ConditionGuestAgentReady)
	g.Expect(ready.Status).To(g.Equal(models.ConditionUnknown))

	vmm, _ := vm.Status.Conditions.Get(models.ConditionVMMRunning)
	g.Expect(vmm.Status).To(g.Equal(models.ConditionUnknown))
}

func TestStartStep_Verify_PollsUntilRunning(t *testing.T) {
	g.RegisterTestingT(t)
	svc := mock.NewMockMicroVMService(gomock.NewController(t))
	vm := testVMToStart()
	ctx := context.Background()

	gomock.InOrder(
		svc.EXPECT().State(ctx, vm.ID.String()).Return(ports.MicroVMStateConfigured, nil),
		svc.EXPECT().State(ctx, vm.ID.String()).Return(ports.MicroVMStateConfigured, nil),
		svc.EXPECT().State(ctx, vm.ID.String()).Return(ports.MicroVMStateRunning, nil),
	)

	g.Expect(microvm.NewStartStep(vm, svc, testWait).Verify(ctx)).To(g.Succeed())

	cond, ok := vm.Status.Conditions.Get(models.ConditionVMMRunning)
	g.Expect(ok).To(g.BeTrue())
	g.Expect(cond.Status).To(g.Equal(models.ConditionTrue))
}

func TestStartStep_Verify_ContextCancelled(t *testing.T) {
	g.RegisterTestingT(t)
	svc := mock.NewMockMicroVMService(gomock.NewController(t))
	vm := testVMToStart()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	svc.EXPECT().State(ctx, vm.ID.String()).Return(ports.MicroVMStateConfigured, nil).AnyTimes()

	step := microvm.NewStartStep(vm, svc, microvm.WaitConfig{
		Timeout:  time.Hour,
		Interval: 10 * time.Millisecond,
		Clock:    testClock,
	})

	start := time.Now()
	err := step.Verify(ctx)

	g.Expect(err).To(g.MatchError(context.Canceled))
	g.Expect(time.Since(start)).To(g.BeNumerically("<", time.Second))
}
