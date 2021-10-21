package plans_test

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"

	"github.com/weaveworks/flintlock/core/models"
	"github.com/weaveworks/flintlock/core/plans"
	"github.com/weaveworks/flintlock/core/ports"
	portsctx "github.com/weaveworks/flintlock/core/ports/context"
)

func TestMicroVMCreatePlan(t *testing.T) {
	RegisterTestingT(t)
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
		State(gomock.Any(), gomock.Eq("namespace/vmid")).
		DoAndReturn(func(_ context.Context, _ string) (ports.MicroVMState, error) {
			return ports.MicroVMStatePending, nil
		}).
		Times(4)

	mList.MicroVMService.
		EXPECT().
		Create(gomock.Any(), gomock.Any())

	mList.MicroVMService.
		EXPECT().
		Start(gomock.Any(), gomock.Eq("namespace/vmid")).
		Return(nil)

	// Start(ctx context.Context, id string) error

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

	Expect(createErr).NotTo(HaveOccurred())
	Expect(steps).To(HaveLen(7))

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
