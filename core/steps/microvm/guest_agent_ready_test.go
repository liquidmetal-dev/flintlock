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
)

const testVSockPath = "/run/flintlock/uid/guest-agent.vsock"

func testGuestAgentVM() *models.MicroVM {
	vm := testVMToStart()
	vm.Spec.AllowGuestAgent = true
	vm.Status.VSockPath = testVSockPath

	return vm
}

func TestGuestAgentReadyStep_ShouldDo(t *testing.T) {
	g.RegisterTestingT(t)

	withReady := func(status models.ConditionStatus) *models.MicroVM {
		vm := testGuestAgentVM()
		vm.Status.Conditions.Set(models.Condition{Type: models.ConditionGuestAgentReady, Status: status}, testNow)

		return vm
	}

	noAgent := testVMToStart()

	testCases := []struct {
		name     string
		vm       *models.MicroVM
		expected bool
	}{
		{name: "no guest-agent", vm: noAgent, expected: false},
		{name: "allowed, condition absent", vm: testGuestAgentVM(), expected: true},
		{name: "allowed, unknown", vm: withReady(models.ConditionUnknown), expected: true},
		{name: "allowed, true", vm: withReady(models.ConditionTrue), expected: false},
		{name: "allowed, false", vm: withReady(models.ConditionFalse), expected: false},
	}

	for _, tc := range testCases {
		ctrl := gomock.NewController(t)
		step := microvm.NewGuestAgentReadyStep(tc.vm, mock.NewMockMicroVMService(ctrl), mock.NewMockGuestAgentService(ctrl), testWait)

		shouldDo, err := step.ShouldDo(context.Background())

		g.Expect(err).NotTo(g.HaveOccurred(), tc.name)
		g.Expect(shouldDo).To(g.Equal(tc.expected), tc.name)
	}
}

func TestGuestAgentReadyStep_Do_Ready(t *testing.T) {
	g.RegisterTestingT(t)
	ctrl := gomock.NewController(t)
	svc := mock.NewMockMicroVMService(ctrl)
	agent := mock.NewMockGuestAgentService(ctrl)
	ctx := context.Background()
	vm := testGuestAgentVM()

	svc.EXPECT().State(ctx, vm.ID.String()).Return(ports.MicroVMStateRunning, nil).AnyTimes()
	gomock.InOrder(
		agent.EXPECT().Ping(ctx, testVSockPath).Return(errors.New("eof")),
		agent.EXPECT().Ping(ctx, testVSockPath).Return(errors.New("eof")),
		agent.EXPECT().Ping(ctx, testVSockPath).Return(nil),
	)

	wait := testWait
	wait.Timeout = time.Second

	_, err := microvm.NewGuestAgentReadyStep(vm, svc, agent, wait).Do(ctx)
	g.Expect(err).NotTo(g.HaveOccurred())

	cond, ok := vm.Status.Conditions.Get(models.ConditionGuestAgentReady)
	g.Expect(ok).To(g.BeTrue())
	g.Expect(cond.Status).To(g.Equal(models.ConditionTrue))
	g.Expect(cond.Reason).To(g.Equal(models.ReasonPingSucceeded))
}

func TestGuestAgentReadyStep_Do_Timeout(t *testing.T) {
	g.RegisterTestingT(t)
	ctrl := gomock.NewController(t)
	svc := mock.NewMockMicroVMService(ctrl)
	agent := mock.NewMockGuestAgentService(ctrl)
	ctx := context.Background()
	vm := testGuestAgentVM()

	svc.EXPECT().State(ctx, vm.ID.String()).Return(ports.MicroVMStateRunning, nil).AnyTimes()
	agent.EXPECT().Ping(ctx, testVSockPath).Return(errors.New("eof")).AnyTimes()

	_, err := microvm.NewGuestAgentReadyStep(vm, svc, agent, testWait).Do(ctx)
	g.Expect(err).NotTo(g.HaveOccurred())

	cond, ok := vm.Status.Conditions.Get(models.ConditionGuestAgentReady)
	g.Expect(ok).To(g.BeTrue())
	g.Expect(cond.Status).To(g.Equal(models.ConditionFalse))
	g.Expect(cond.Reason).To(g.Equal(models.ReasonPingTimeout))
	g.Expect(cond.Message).To(g.ContainSubstring("eof"))
}

func TestGuestAgentReadyStep_Do_SetsMissingVMMRunning(t *testing.T) {
	g.RegisterTestingT(t)
	ctrl := gomock.NewController(t)
	svc := mock.NewMockMicroVMService(ctrl)
	agent := mock.NewMockGuestAgentService(ctrl)
	ctx := context.Background()
	vm := testGuestAgentVM()

	svc.EXPECT().State(ctx, vm.ID.String()).Return(ports.MicroVMStateRunning, nil)
	agent.EXPECT().Ping(ctx, testVSockPath).Return(nil)

	_, err := microvm.NewGuestAgentReadyStep(vm, svc, agent, testWait).Do(ctx)
	g.Expect(err).NotTo(g.HaveOccurred())

	vmm, ok := vm.Status.Conditions.Get(models.ConditionVMMRunning)
	g.Expect(ok).To(g.BeTrue())
	g.Expect(vmm.Status).To(g.Equal(models.ConditionTrue))
	g.Expect(vmm.Reason).To(g.Equal(models.ReasonRunning))

	ready, _ := vm.Status.Conditions.Get(models.ConditionGuestAgentReady)
	g.Expect(ready.Status).To(g.Equal(models.ConditionTrue))
}

func TestGuestAgentReadyStep_Do_VMMExited(t *testing.T) {
	g.RegisterTestingT(t)
	ctrl := gomock.NewController(t)
	svc := mock.NewMockMicroVMService(ctrl)
	agent := mock.NewMockGuestAgentService(ctrl)
	ctx := context.Background()
	vm := testGuestAgentVM()

	gomock.InOrder(
		svc.EXPECT().State(ctx, vm.ID.String()).Return(ports.MicroVMStateRunning, nil),
		svc.EXPECT().State(ctx, vm.ID.String()).Return(ports.MicroVMStatePending, nil),
	)
	agent.EXPECT().Ping(ctx, testVSockPath).Return(errors.New("eof")).AnyTimes()

	wait := testWait
	wait.Timeout = time.Second

	_, err := microvm.NewGuestAgentReadyStep(vm, svc, agent, wait).Do(ctx)
	g.Expect(err).To(g.MatchError(internalerr.ErrUnableToBoot))

	vmm, _ := vm.Status.Conditions.Get(models.ConditionVMMRunning)
	g.Expect(vmm.Status).To(g.Equal(models.ConditionFalse))
	g.Expect(vmm.Reason).To(g.Equal(models.ReasonVMMExited))

	ready, _ := vm.Status.Conditions.Get(models.ConditionGuestAgentReady)
	g.Expect(ready.Status).To(g.Equal(models.ConditionFalse))
	g.Expect(ready.Reason).To(g.Equal(models.ReasonVMMExited))
}

func TestGuestAgentReadyStep_Do_ContextCancelled(t *testing.T) {
	g.RegisterTestingT(t)
	ctrl := gomock.NewController(t)
	svc := mock.NewMockMicroVMService(ctrl)
	agent := mock.NewMockGuestAgentService(ctrl)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	vm := testGuestAgentVM()

	svc.EXPECT().State(ctx, vm.ID.String()).Return(ports.MicroVMStateRunning, nil).AnyTimes()
	agent.EXPECT().Ping(ctx, testVSockPath).Return(errors.New("canceled")).AnyTimes()

	wait := testWait
	wait.Timeout = time.Hour

	_, err := microvm.NewGuestAgentReadyStep(vm, svc, agent, wait).Do(ctx)
	g.Expect(err).To(g.MatchError(context.Canceled))

	_, ok := vm.Status.Conditions.Get(models.ConditionGuestAgentReady)
	g.Expect(ok).To(g.BeFalse())
}
