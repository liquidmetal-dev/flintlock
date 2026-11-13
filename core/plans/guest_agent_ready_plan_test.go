package plans_test

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/plans"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	portsctx "github.com/liquidmetal-dev/flintlock/core/ports/context"
	"github.com/liquidmetal-dev/flintlock/pkg/planner"
)

const guestAgentReadyStepName = "microvm_guest_agent_ready"

func minimalTestSpec(allowGuestAgent bool) *models.MicroVM {
	vmid, _ := models.NewVMID("vmid", "namespace", testUID)

	return &models.MicroVM{
		ID: *vmid,
		Status: models.MicroVMStatus{
			State:   models.PendingState,
			Volumes: models.VolumeStatuses{},
		},
		Spec: models.MicroVMSpec{
			Provider:        "mock",
			VCPU:            1,
			MemoryInMb:      512,
			RootVolume:      models.Volume{ID: "root"},
			AllowGuestAgent: allowGuestAgent,
		},
	}
}

func newPlanWithPorts(t *testing.T, vm *models.MicroVM, state ports.MicroVMState) (planner.Plan, context.Context, *mockList) {
	t.Helper()

	mList, mockedPorts := fakePorts(gomock.NewController(t))
	ctx := portsctx.WithPorts(context.Background(), mockedPorts)

	mList.MicroVMService.EXPECT().State(gomock.Any(), gomock.Any()).Return(state, nil).AnyTimes()
	mList.MicroVMService.EXPECT().Capabilities().Return(models.Capabilities{}).AnyTimes()

	plan := plans.MicroVMCreateOrUpdatePlan(&plans.CreateOrUpdatePlanInput{
		VM:              vm,
		StateDirectory:  "/tmp/path/to/vm",
		SocketDirectory: "/tmp/run",
	})

	return plan, ctx, mList
}

func TestCreatePlan_GuestAgentReadyStepIsLast(t *testing.T) {
	RegisterTestingT(t)

	plan, ctx, _ := newPlanWithPorts(t, minimalTestSpec(true), ports.MicroVMStatePending)

	steps, err := plan.Create(ctx)
	Expect(err).NotTo(HaveOccurred())

	names := stepNames(steps)
	Expect(names).NotTo(BeEmpty())
	Expect(names[len(names)-1]).To(Equal(guestAgentReadyStepName))
}

func TestCreatePlan_NoGuestAgentNoReadyStep(t *testing.T) {
	RegisterTestingT(t)

	plan, ctx, _ := newPlanWithPorts(t, minimalTestSpec(false), ports.MicroVMStatePending)

	steps, err := plan.Create(ctx)
	Expect(err).NotTo(HaveOccurred())
	Expect(stepNames(steps)).NotTo(ContainElement(guestAgentReadyStepName))
}

func TestCreatePlan_RestartPingsAgain(t *testing.T) {
	RegisterTestingT(t)

	vm := minimalTestSpec(true)
	vm.Status.Conditions.Set(models.Condition{
		Type:   models.ConditionGuestAgentReady,
		Status: models.ConditionTrue,
	}, testNowPlans)

	plan, ctx, mList := newPlanWithPorts(t, vm, ports.MicroVMStatePending)

	steps, err := plan.Create(ctx)
	Expect(err).NotTo(HaveOccurred())
	Expect(stepNames(steps)).To(ContainElement("microvm_create"))
	Expect(stepNames(steps)).NotTo(ContainElement(guestAgentReadyStepName))

	mList.MicroVMService.EXPECT().Create(gomock.Any(), vm).Return(nil)

	for _, step := range steps {
		if step.Name() == "microvm_create" {
			_, err := step.Do(ctx)
			Expect(err).NotTo(HaveOccurred())
		}
	}

	steps, err = plan.Create(ctx)
	Expect(err).NotTo(HaveOccurred())
	Expect(stepNames(steps)).To(ContainElement(guestAgentReadyStepName))
}
