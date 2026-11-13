package microvm_test

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	g "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/core/steps/microvm"
	"github.com/liquidmetal-dev/flintlock/infrastructure/mock"
)

func TestVMMRunningStep_ShouldDo(t *testing.T) {
	tt := []struct {
		name     string
		state    ports.MicroVMState
		existing bool
		expected bool
	}{
		{name: "running vmm without the condition", state: ports.MicroVMStateRunning, expected: true},
		{name: "running vmm with the condition", state: ports.MicroVMStateRunning, existing: true, expected: false},
		{name: "vmm that does not run", state: ports.MicroVMStateConfigured, expected: false},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			g.RegisterTestingT(t)
			svc := mock.NewMockMicroVMService(gomock.NewController(t))
			vm := testVMToStart()
			ctx := context.Background()

			if tc.existing {
				vm.Status.Conditions.Set(models.Condition{
					Type:   models.ConditionVMMRunning,
					Status: models.ConditionFalse,
				}, testNow)
			}

			svc.EXPECT().State(ctx, vm.ID.String()).Return(tc.state, nil).AnyTimes()

			shouldDo, err := microvm.NewVMMRunningStep(vm, svc, testWait).ShouldDo(ctx)

			g.Expect(err).NotTo(g.HaveOccurred())
			g.Expect(shouldDo).To(g.Equal(tc.expected))
		})
	}
}

func TestVMMRunningStep_Do_SetsVMMRunningTrue(t *testing.T) {
	g.RegisterTestingT(t)
	svc := mock.NewMockMicroVMService(gomock.NewController(t))
	vm := testVMToStart()
	ctx := context.Background()

	subSteps, err := microvm.NewVMMRunningStep(vm, svc, testWait).Do(ctx)

	g.Expect(err).NotTo(g.HaveOccurred())
	g.Expect(subSteps).To(g.BeEmpty())

	cond, ok := vm.Status.Conditions.Get(models.ConditionVMMRunning)
	g.Expect(ok).To(g.BeTrue())
	g.Expect(cond.Status).To(g.Equal(models.ConditionTrue))
	g.Expect(cond.Reason).To(g.Equal(models.ReasonRunning))
	g.Expect(cond.LastTransition).To(g.Equal(testNow))
}
