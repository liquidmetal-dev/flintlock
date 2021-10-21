package plans_test

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/reignite/core/models"
	"github.com/weaveworks/reignite/core/plans"
	"github.com/weaveworks/reignite/core/ports"
	portsctx "github.com/weaveworks/reignite/core/ports/context"
)

func TestMicroVMCreatePlan(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mList, mockedPorts := fakePorts(mockCtrl)
	ctx := portsctx.WithPorts(
		context.Background(),
		mockedPorts,
	)
	plan := plans.MicroVMCreatePlan(&plans.CreatePlanInput{
		VM:             createTestSpec("vmid", "namespace"),
		StateDirectory: "/tmp/path/to/vm",
	})

	mList.MicroVMService.
		EXPECT().
		IsRunning(gomock.Any(), gomock.Eq("namespace/vmid")).
		DoAndReturn(func(_ context.Context, _ string) (bool, error) {
			return false, nil
		}).
		Times(2)

	mList.MicroVMService.
		EXPECT().
		Create(gomock.Any(), gomock.Any())

	mList.NetworkService.
		EXPECT().
		IfaceExists(gomock.Any(), gomock.Eq("namespace_vmid_tap")).
		DoAndReturn(func(_ context.Context, _ string) (bool, error) {
			return false, nil
		}).
		Times(4)

	mList.NetworkService.
		EXPECT().
		IfaceCreate(
			gomock.Any(),
			gomock.Eq(ports.IfaceCreateInput{
				DeviceName: "namespace_vmid_tap",
				MAC:        "AA:FF:00:00:00:01",
			}),
		).
		Return(&ports.IfaceDetails{}, nil)

	mList.NetworkService.
		EXPECT().
		IfaceCreate(
			gomock.Any(),
			gomock.Eq(ports.IfaceCreateInput{
				DeviceName: "namespace_vmid_tap",
			}),
		).
		Return(&ports.IfaceDetails{}, nil)

	mList.ImageService.
		EXPECT().
		PullAndMount(
			gomock.Any(),
			gomock.Eq(&ports.ImageMountSpec{
				ImageName:    "docker.io/library/ubuntu:myimage",
				Owner:        "namespace/vmid",
				OwnerUsageID: "root",
				Use:          "volume",
			}),
		).
		Return([]models.Mount{{Type: models.MountTypeHostPath}}, nil).
		Times(1)

	mList.ImageService.
		EXPECT().
		PullAndMount(
			gomock.Any(),
			gomock.Eq(&ports.ImageMountSpec{
				ImageName:    "docker.io/linuxkit/kernel:5.4.129",
				Owner:        "namespace/vmid",
				OwnerUsageID: "kernel",
				Use:          "kernel",
			}),
		).
		Return([]models.Mount{{Type: models.MountTypeHostPath}}, nil).
		Times(1)

	steps, createErr := plan.Create(ctx)

	assert.NoError(t, createErr)
	assert.Equal(t, 6, len(steps))

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
