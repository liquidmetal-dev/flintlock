package plans_test

import (
	"context"
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/reignite/core/plans"
	"github.com/weaveworks/reignite/core/ports"
	portsctx "github.com/weaveworks/reignite/core/ports/context"
)

func TestMicroVMDeletePlan(t *testing.T) {
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
		IsRunning(gomock.Any(), gomock.Eq("namespace/vmid")).
		Return(true, nil).
		Times(2)

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

	steps, createErr := plan.Create(ctx)

	assert.NoError(t, createErr)
	assert.Equal(t, 3, len(steps))

	for _, step := range steps {
		should, err := step.ShouldDo(ctx)

		assert.NoError(t, err)
		assert.True(t, should)

		if should {
			extraSteps, err := step.Do(ctx)

			assert.NoError(t, err)
			assert.Nil(t, extraSteps)
		}
	}
}
