package plans_test

import (
	"context"
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"

	"github.com/weaveworks/flintlock/core/plans"
	"github.com/weaveworks/flintlock/core/ports"
	portsctx "github.com/weaveworks/flintlock/core/ports/context"
	"github.com/weaveworks/flintlock/pkg/defaults"
)

func TestMicroVMDeletePlan(t *testing.T) {
	RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mList, mockedPorts := fakePorts(mockCtrl)
	ctx := portsctx.WithPorts(
		context.Background(),
		mockedPorts,
	)
	spec := createTestSpec("vmid", "namespace")
	spec.Spec.DeletedAt = 1
	plan := plans.MicroVMDeletePlan(&plans.DeletePlanInput{
		VM:             spec,
		StateDirectory: "/tmp/path/to/vm",
	})

	mockedPorts.FileSystem.MkdirAll("/tmp/path/to/vm/asd", os.ModeDir)

	mList.MicroVMService.
		EXPECT().
		State(gomock.Any(), gomock.Eq("namespace/vmid")).
		DoAndReturn(func(_ context.Context, _ string) (ports.MicroVMState, error) {
			return ports.MicroVMStateRunning, nil
		}).AnyTimes()

	mList.MicroVMRepository.
		EXPECT().
		Exists(gomock.Any(), gomock.Eq("vmid"), gomock.Eq("namespace")).
		Return(true, nil).
		AnyTimes()

	mList.MicroVMService.
		EXPECT().
		Delete(gomock.Any(), "namespace/vmid").
		Return(nil).
		Times(1)

	mList.NetworkService.
		EXPECT().
		IfaceExists(gomock.Any(), gomock.Eq("namespace_vmid_tap")).
		Return(true, nil).
		AnyTimes()

	mList.NetworkService.
		EXPECT().
		IfaceDelete(
			gomock.Any(),
			ports.DeleteIfaceInput{DeviceName: "namespace_vmid_tap"},
		).
		Times(2)

	mList.MicroVMRepository.
		EXPECT().
		ReleaseLease(gomock.Any(), gomock.Any()).
		Return(nil).
		AnyTimes()

	mList.EventService.
		EXPECT().
		Publish(gomock.Any(), gomock.Eq(defaults.TopicMicroVMEvents), gomock.Any()).
		Return(nil).
		AnyTimes()

	steps, createErr := plan.Create(ctx)

	Expect(createErr).NotTo(HaveOccurred())
	Expect(steps).To(HaveLen(5))

	for _, step := range steps {
		should, err := step.ShouldDo(ctx)

		Expect(err).NotTo(HaveOccurred())
		Expect(should).To(BeTrue())

		if should {
			extraSteps, err := step.Do(ctx)

			Expect(err).NotTo(HaveOccurred())
			Expect(extraSteps).To(BeNil())
		}
	}
}
