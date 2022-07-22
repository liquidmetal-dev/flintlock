package plans_test

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"

	"github.com/weaveworks-liquidmetal/flintlock/client/cloudinit"
	"github.com/weaveworks-liquidmetal/flintlock/core/models"
	"github.com/weaveworks-liquidmetal/flintlock/core/plans"
	"github.com/weaveworks-liquidmetal/flintlock/core/ports"
	portsctx "github.com/weaveworks-liquidmetal/flintlock/core/ports/context"
	"github.com/weaveworks-liquidmetal/flintlock/pkg/planner"
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
		VM:             testVM,
		StateDirectory: "/tmp/path/to/vm",
	})

	mList.MicroVMService.
		EXPECT().
		State(gomock.Any(), gomock.Eq("namespace/vmid/ae1ce196-6249-11ec-90d6-0242ac120003")).
		DoAndReturn(func(_ context.Context, _ string) (ports.MicroVMState, error) {
			return ports.MicroVMStatePending, nil
		}).
		Times(4)

	mList.MicroVMService.
		EXPECT().
		Create(gomock.Any(), gomock.Any())

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
	executeSteps(ctx, steps)
}

func TestMicroVMCreateOrUpdatePlanWithExtraVolumes(t *testing.T) {
	RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	testVM := createTestSpec("vmid", "namespace")
	testVM.Spec.Metadata = models.Metadata{
		Items: map[string]string{
			cloudinit.InstanceDataKey: "aW5zdGFuY2VfaWQ6IDEyMzQ1Ngo=",
			"custom":                  "dGhpcyBpcyBhIHRlc3Q=",
		},
		AddVolume: true,
	}
	mList, mockedPorts := fakePorts(mockCtrl)
	ctx := portsctx.WithPorts(
		context.Background(),
		mockedPorts,
	)
	plan := plans.MicroVMCreateOrUpdatePlan(&plans.CreateOrUpdatePlanInput{
		VM:                 testVM,
		StateDirectory:     "/tmp/path/to/vm",
		CloudinitViaVolume: true,
	})

	mList.MicroVMService.
		EXPECT().
		State(gomock.Any(), gomock.Eq("namespace/vmid/ae1ce196-6249-11ec-90d6-0242ac120003")).
		DoAndReturn(func(_ context.Context, _ string) (ports.MicroVMState, error) {
			return ports.MicroVMStatePending, nil
		}).
		Times(4)

	mList.MicroVMService.
		EXPECT().
		Create(gomock.Any(), gomock.Any())

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

	mList.DiskService.EXPECT().Create(
		gomock.Any(),
		&diskCreateInputMatcher{
			Expected: &ports.DiskCreateInput{
				Path:       "/tmp/path/to/vm/vm/namespace/vmid/ae1ce196-6249-11ec-90d6-0242ac120003/data.img",
				Size:       "8Mb",
				VolumeName: "data",
				Type:       ports.DiskTypeFat32,
				Files: []ports.DiskFile{
					{
						Path:          "/custom",
						ContentBase64: "dGhpcyBpcyBhIHRlc3Q=",
					},
				},
			}}).Return(nil).Times(1)

	mList.DiskService.EXPECT().Create(
		gomock.Any(),
		&diskCreateInputMatcher{
			Expected: &ports.DiskCreateInput{
				Path:       "/tmp/path/to/vm/vm/namespace/vmid/ae1ce196-6249-11ec-90d6-0242ac120003/cloudinit.img",
				Size:       "8Mb",
				VolumeName: "cidata",
				Type:       ports.DiskTypeFat32,
				Files: []ports.DiskFile{
					{
						Path:          "/vendor-data",
						ContentBase64: "IyMgdGVtcGxhdGU6IGppbmphCiNjbG91ZC1jb25maWcKCm1vdW50czoKLSAtIHZkYjIKICAtIC9vcHQvZGF0YQptb3VudF9kZWZhdWx0X2ZpZWxkczogW05vbmUsIE5vbmUsIGF1dG8sICdkZWZhdWx0cyxub2ZhaWwnLCAiMCIsICIyIl0K",
					},
					{
						Path:          "/meta-data",
						ContentBase64: "aW5zdGFuY2VfaWQ6IDEyMzQ1Ngo=",
					},
				},
			}}).Return(nil).Times(1)

	steps, createErr := plan.Create(ctx)

	Expect(createErr).NotTo(HaveOccurred())
	Expect(steps).To(HaveLen(10))

	Expect(testVM.Status.State).To(Equal(models.MicroVMState(models.PendingState)))
	executeSteps(ctx, steps)
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
			VM:             vm,
			StateDirectory: "/tmp/path/to/vm",
		})

		plan.Finalise(tc.state)

		Expect(vm.Status.State).To(Equal(models.MicroVMState(tc.state)))
	}
}

func executeSteps(ctx context.Context, steps []planner.Procedure) {
	for _, step := range steps {
		should, err := step.ShouldDo(ctx)

		Expect(err).NotTo(HaveOccurred())
		Expect(should).To(BeTrue())

		if should {
			extraSteps, err := step.Do(ctx)
			Expect(err).NotTo(HaveOccurred())

			if len(extraSteps) > 0 {
				executeSteps(ctx, extraSteps)
			}
		}
	}
}
