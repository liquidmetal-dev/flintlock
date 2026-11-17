package plans_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
)

const pauseStepName = "microvm_pause"

// pausedTestSpec returns a created microvm whose client wants it paused.
func pausedTestSpec() *models.MicroVM {
	vm := minimalTestSpec(false)
	vm.Status.State = models.PausedState
	vm.Spec.PowerState = models.PowerStatePaused
	vm.Status.Conditions.Set(models.Condition{
		Type:   models.ConditionVMMRunning,
		Status: models.ConditionTrue,
		Reason: models.ReasonRunning,
	}, testNowPlans)

	return vm
}

func TestCreatePlan_PausedVMMThatWantsPausedHasNoSteps(t *testing.T) {
	RegisterTestingT(t)

	plan, ctx, _ := newPlanWithPorts(t, pausedTestSpec(), ports.MicroVMStatePaused)

	steps, err := plan.Create(ctx)
	Expect(err).NotTo(HaveOccurred())
	for _, name := range []string{"microvm_create", "microvm_start", pauseStepName} {
		Expect(stepNames(steps)).NotTo(ContainElement(name))
	}
}

func TestCreatePlan_RunningVMMThatWantsPausedIsPausedLast(t *testing.T) {
	RegisterTestingT(t)

	plan, ctx, _ := newPlanWithPorts(t, pausedTestSpec(), ports.MicroVMStateRunning)

	steps, err := plan.Create(ctx)
	Expect(err).NotTo(HaveOccurred())

	names := stepNames(steps)
	Expect(names).NotTo(BeEmpty())
	Expect(names[len(names)-1]).To(Equal(pauseStepName))
}

func TestCreatePlan_PausedVMMThatWantsRunningIsNotTouched(t *testing.T) {
	RegisterTestingT(t)

	vm := pausedTestSpec()
	vm.Spec.PowerState = models.PowerStateRunning
	vm.Status.State = models.CreatedState

	plan, ctx, _ := newPlanWithPorts(t, vm, ports.MicroVMStatePaused)

	steps, err := plan.Create(ctx)
	Expect(err).NotTo(HaveOccurred())
	for _, name := range []string{"microvm_create", "microvm_start", pauseStepName} {
		Expect(stepNames(steps)).NotTo(ContainElement(name))
	}
}
