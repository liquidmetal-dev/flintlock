package planner_test

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/infrastructure/ulid"
	"github.com/liquidmetal-dev/flintlock/pkg/planner"
)

func TestActuator_SingleProc(t *testing.T) {
	RegisterTestingT(t)

	ctx := context.Background()

	testProcs := []planner.Procedure{newTestProc(10*time.Millisecond, []planner.Procedure{})}
	testPlan := newTestPlan(testProcs)

	idSrv := ulid.New()
	execID, err := idSrv.GenerateRandom()
	Expect(err).NotTo(HaveOccurred())

	act := planner.NewActuator()
	stepCount, err := act.Execute(ctx, testPlan, execID)

	Expect(err).NotTo(HaveOccurred())
	testProc, ok := testProcs[0].(*testProc)
	Expect(ok).To(BeTrue())
	Expect(testProc.Executed).To(BeTrue())
	Expect(stepCount).To(Equal(1))
}

func TestActuator_MultipleProcs(t *testing.T) {
	RegisterTestingT(t)

	ctx := context.Background()

	testProcs := []planner.Procedure{newTestProc(10*time.Millisecond, []planner.Procedure{}), newTestProc(10*time.Millisecond, []planner.Procedure{})}
	testPlan := newTestPlan(testProcs)

	idSrv := ulid.New()
	execID, err := idSrv.GenerateRandom()
	Expect(err).NotTo(HaveOccurred())

	act := planner.NewActuator()
	stepCount, err := act.Execute(ctx, testPlan, execID)
	Expect(err).NotTo(HaveOccurred())
	Expect(stepCount).To(Equal(2))

	for _, proc := range testProcs {
		testProc, ok := proc.(*testProc)
		Expect(ok).To(BeTrue())
		Expect(testProc.Executed).To(BeTrue())
	}
}

func TestActuator_ChildProcs(t *testing.T) {
	RegisterTestingT(t)

	ctx := context.Background()

	testProcs := []planner.Procedure{newTestProc(10*time.Millisecond, []planner.Procedure{newTestProc(10*time.Millisecond, []planner.Procedure{})})}
	testPlan := newTestPlan(testProcs)

	idSrv := ulid.New()
	execID, err := idSrv.GenerateRandom()
	Expect(err).NotTo(HaveOccurred())

	act := planner.NewActuator()
	stepCount, err := act.Execute(ctx, testPlan, execID)
	Expect(err).NotTo(HaveOccurred())
	Expect(stepCount).To(Equal(2))

	parentProc, ok := testProcs[0].(*testProc)
	Expect(ok).To(BeTrue())
	Expect(parentProc.Executed).To(BeTrue())

	childProc, ok := parentProc.ChildProcs[0].(*testProc)
	Expect(ok).To(BeTrue())
	Expect(childProc.Executed).To(BeTrue())
}

func TestActuator_Timeout(t *testing.T) {
	RegisterTestingT(t)

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()

	testProcs := []planner.Procedure{
		newTestProc(210*time.Millisecond, []planner.Procedure{}),
		newTestProc(200*time.Millisecond, []planner.Procedure{}),
	}
	testPlan := newTestPlan(testProcs)

	idSrv := ulid.New()
	execID, err := idSrv.GenerateRandom()
	Expect(err).NotTo(HaveOccurred())

	act := planner.NewActuator()
	stepCount, err := act.Execute(ctx, testPlan, execID)
	Expect(stepCount).To(Equal(1))

	Expect(err).To(HaveOccurred())
	Expect(err).To(MatchError(context.DeadlineExceeded))

	proc1, ok := testProcs[0].(*testProc)
	Expect(ok).To(BeTrue())
	Expect(proc1.Executed).To(BeTrue())

	proc2, ok := testProcs[1].(*testProc)
	Expect(ok).To(BeTrue())
	Expect(proc2.Executed).To(BeFalse())
}

func newTestPlan(procs []planner.Procedure) planner.Plan {
	return &testPlan{
		testProcs: procs,
	}
}

type testPlan struct {
	testProcs []planner.Procedure
}

func (tp *testPlan) Name() string {
	return "test_plan"
}

func (tp *testPlan) Create(ctx context.Context) ([]planner.Procedure, error) {
	toExec := []planner.Procedure{}

	for _, proc := range tp.testProcs {
		testProc, _ := proc.(*testProc)
		if !testProc.Executed {
			toExec = append(toExec, proc)
		}
	}
	return toExec, nil
}

func (tp *testPlan) Finalise(_ models.MicroVMState) {
}

func newTestProc(delay time.Duration, childProcs []planner.Procedure) planner.Procedure {
	return &testProc{
		DoDelay:    delay,
		ChildProcs: childProcs,
	}
}

type testProc struct {
	DoDelay    time.Duration
	ChildProcs []planner.Procedure
	Executed   bool
}

func (p *testProc) Name() string {
	return "test_proc"
}

func (p *testProc) Do(ctx context.Context) ([]planner.Procedure, error) {
	p.Executed = true
	time.Sleep(p.DoDelay)

	return p.ChildProcs, nil
}

func (p *testProc) ShouldDo(ctx context.Context) (bool, error) {
	return true, nil
}

func (p *testProc) Verify(ctx context.Context) error {
	return nil
}
