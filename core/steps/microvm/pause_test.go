package microvm_test

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	g "github.com/onsi/gomega"

	internalerr "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/core/steps/microvm"
	"github.com/liquidmetal-dev/flintlock/infrastructure/mock"
)

func TestPauseStep_ShouldDo(t *testing.T) {
	tt := []struct {
		name     string
		power    models.PowerState
		state    ports.MicroVMState
		expected bool
	}{
		{name: "wants paused, vmm runs", power: models.PowerStatePaused, state: ports.MicroVMStateRunning, expected: true},
		{name: "wants paused, vmm paused", power: models.PowerStatePaused, state: ports.MicroVMStatePaused},
		{name: "wants paused, vmm not started", power: models.PowerStatePaused, state: ports.MicroVMStatePending},
		{name: "wants running, vmm runs", power: models.PowerStateRunning, state: ports.MicroVMStateRunning},
		{name: "no power state, vmm runs", state: ports.MicroVMStateRunning},
		// A snapshot pauses the vmm while it captures it. The step must not touch that.
		{name: "wants running, vmm paused", power: models.PowerStateRunning, state: ports.MicroVMStatePaused},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			g.RegisterTestingT(t)
			svc := mock.NewMockMicroVMService(gomock.NewController(t))
			vm := testVMToStart()
			vm.Spec.PowerState = tc.power
			ctx := context.Background()

			svc.EXPECT().State(ctx, vm.ID.String()).Return(tc.state, nil).AnyTimes()

			shouldDo, err := microvm.NewPauseStep(vm, svc).ShouldDo(ctx)

			g.Expect(err).NotTo(g.HaveOccurred())
			g.Expect(shouldDo).To(g.Equal(tc.expected))
		})
	}
}

func TestPauseStep_ShouldDo_StateError(t *testing.T) {
	g.RegisterTestingT(t)
	svc := mock.NewMockMicroVMService(gomock.NewController(t))
	vm := testVMToStart()
	vm.Spec.PowerState = models.PowerStatePaused
	ctx := context.Background()

	svc.EXPECT().State(ctx, vm.ID.String()).Return(ports.MicroVMStateUnknown, errors.New("no idea"))

	_, err := microvm.NewPauseStep(vm, svc).ShouldDo(ctx)

	g.Expect(err).To(g.HaveOccurred())
}

func TestPauseStep_DoAndVerify(t *testing.T) {
	g.RegisterTestingT(t)
	svc := mock.NewMockMicroVMService(gomock.NewController(t))
	vm := testVMToStart()
	vm.Spec.PowerState = models.PowerStatePaused
	ctx := context.Background()

	pause := svc.EXPECT().Pause(ctx, vm).Return(nil)
	svc.EXPECT().State(ctx, vm.ID.String()).Return(ports.MicroVMStatePaused, nil).After(pause)

	step := microvm.NewPauseStep(vm, svc)
	subSteps, err := step.Do(ctx)

	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(subSteps).To(g.BeEmpty())
	g.Expect(step.Verify(ctx)).To(g.Succeed())
}

func TestPauseStep_DoError(t *testing.T) {
	g.RegisterTestingT(t)
	svc := mock.NewMockMicroVMService(gomock.NewController(t))
	vm := testVMToStart()
	vm.Spec.PowerState = models.PowerStatePaused
	ctx := context.Background()

	svc.EXPECT().Pause(ctx, vm).Return(errors.New("pause failed"))

	_, err := microvm.NewPauseStep(vm, svc).Do(ctx)

	g.Expect(err).To(g.MatchError(g.ContainSubstring("pause failed")))
}

func TestPauseStep_VerifyFailsWhenVMMRuns(t *testing.T) {
	g.RegisterTestingT(t)
	svc := mock.NewMockMicroVMService(gomock.NewController(t))
	vm := testVMToStart()
	ctx := context.Background()

	svc.EXPECT().State(ctx, vm.ID.String()).Return(ports.MicroVMStateRunning, nil)

	g.Expect(microvm.NewPauseStep(vm, svc).Verify(ctx)).To(g.MatchError(g.ContainSubstring("not paused")))
}

func TestPauseStep_VMIsNotDefined(t *testing.T) {
	g.RegisterTestingT(t)
	svc := mock.NewMockMicroVMService(gomock.NewController(t))

	_, err := microvm.NewPauseStep(nil, svc).Do(context.Background())

	g.Expect(err).To(g.MatchError(internalerr.ErrSpecRequired))
}
