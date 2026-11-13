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
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
)

func TestMicroVMCreateOrUpdatePlan(t *testing.T) {
	RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	testVM := createTestSpec("vmid", "namespace")
	mList, mockedPorts := fakePorts(mockCtrl)
	ctx := portsctx.WithPorts(
		context.Background(),
		mockedPorts,
	)
	plan := plans.MicroVMCreateOrUpdatePlan(&plans.CreateOrUpdatePlanInput{
		VM:              testVM,
		StateDirectory:  "/tmp/path/to/vm",
		SocketDirectory: "/tmp/run",
	})

	mList.MicroVMService.
		EXPECT().
		State(gomock.Any(), gomock.Eq("namespace/vmid/ae1ce196-6249-11ec-90d6-0242ac120003")).
		DoAndReturn(func(_ context.Context, _ string) (ports.MicroVMState, error) {
			return ports.MicroVMStatePending, nil
		}).
		Times(5)

	mList.MicroVMService.
		EXPECT().
		Create(gomock.Any(), gomock.Any())

	mList.MicroVMService.EXPECT().Capabilities().Return(models.Capabilities{}).Times(2)

	mList.MicroVMService.
		EXPECT().
		Start(gomock.Any(), gomock.Any()).
		Return(nil)

	mList.NetworkService.
		EXPECT().
		IfaceExists(gomock.Any(), &hostDeviceNameMatcher{}).
		Return(false, nil).
		Times(4)

	mList.NetworkService.
		EXPECT().
		IfaceCreate(
			gomock.Any(),
			&createInterfaceMatcher{
				MAC:  "AA:FF:00:00:00:01",
				Type: models.IfaceTypeTap,
			},
		).
		Return(&ports.IfaceDetails{}, nil)

	mList.NetworkService.
		EXPECT().
		IfaceCreate(
			gomock.Any(),
			&createInterfaceMatcher{
				Type: models.IfaceTypeTap,
			},
		).
		Return(&ports.IfaceDetails{}, nil)

	mList.ImageService.
		EXPECT().
		PullAndMount(
			gomock.Any(),
			gomock.Eq(&ports.ImageMountSpec{
				ImageName:    "docker.io/library/ubuntu:myimage",
				Owner:        "namespace/vmid/ae1ce196-6249-11ec-90d6-0242ac120003",
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
				Owner:        "namespace/vmid/ae1ce196-6249-11ec-90d6-0242ac120003",
				OwnerUsageID: "kernel",
				Use:          "kernel",
			}),
		).
		Return([]models.Mount{{Type: models.MountTypeHostPath}}, nil).
		Times(1)

	steps, createErr := plan.Create(ctx)

	Expect(createErr).NotTo(HaveOccurred())
	Expect(steps).To(HaveLen(8))

	Expect(testVM.Status.State).To(Equal(models.MicroVMState(models.PendingState)))

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

func TestMicroVMPlanFinalise(t *testing.T) {
	tt := []struct {
		name  string
		state models.MicroVMState
	}{
		{
			name:  "finalise with created updates mvm state to created",
			state: models.CreatedState,
		},
		{
			name:  "finalise with failed updates mvm state to created",
			state: models.FailedState,
		},
	}
	for _, tc := range tt {
		RegisterTestingT(t)
		vm := createTestSpec("vmid", "namespace")
		plan := plans.MicroVMCreateOrUpdatePlan(&plans.CreateOrUpdatePlanInput{
			VM:              vm,
			StateDirectory:  "/tmp/path/to/vm",
			SocketDirectory: "/tmp/run",
		})

		plan.Finalise(tc.state)

		Expect(vm.Status.State).To(Equal(models.MicroVMState(tc.state)))
	}
}

func TestMicroVMCreateOrUpdatePlan_PausedMicroVMHasNoSteps(t *testing.T) {
	RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	testVM := createTestSpec("vmid", "namespace")
	testVM.Status.Volumes = models.VolumeStatuses{
		"root": &models.VolumeStatus{Mount: models.Mount{Type: models.MountTypeHostPath, Source: "/root"}},
	}
	testVM.Status.KernelMount = &models.Mount{Type: models.MountTypeHostPath, Source: "/kernel"}
	testVM.Status.NetworkInterfaces["eth1"] = &models.NetworkInterfaceStatus{HostDeviceName: "fltap5675123"}
	testVM.Status.Conditions.Set(models.Condition{
		Type:   models.ConditionVMMRunning,
		Status: models.ConditionTrue,
		Reason: models.ReasonRunning,
	}, testNowPlans)

	mList, mockedPorts := fakePorts(mockCtrl)
	Expect(mockedPorts.FileSystem.MkdirAll("/tmp/path/to/vm", defaults.DataDirPerm)).To(Succeed())
	Expect(mockedPorts.FileSystem.MkdirAll(models.SocketRoot("/tmp/run", testVM.ID), defaults.DataDirPerm)).To(Succeed())

	ctx := portsctx.WithPorts(context.Background(), mockedPorts)
	plan := plans.MicroVMCreateOrUpdatePlan(&plans.CreateOrUpdatePlanInput{
		VM:              testVM,
		StateDirectory:  "/tmp/path/to/vm",
		SocketDirectory: "/tmp/run",
	})

	mList.MicroVMService.EXPECT().Capabilities().Return(models.Capabilities{}).Times(2)
	mList.MicroVMService.
		EXPECT().
		State(gomock.Any(), gomock.Eq(testVM.ID.String())).
		Return(ports.MicroVMStatePaused, nil).
		Times(2)
	mList.ImageService.EXPECT().IsMounted(gomock.Any(), gomock.Any()).Return(true, nil).Times(2)
	mList.NetworkService.EXPECT().IfaceExists(gomock.Any(), gomock.Any()).Return(true, nil).Times(2)

	steps, err := plan.Create(ctx)

	Expect(err).NotTo(HaveOccurred())
	Expect(steps).To(BeEmpty())
}
