package plans_test

import (
	"context"
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"

	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/plans"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	portsctx "github.com/liquidmetal-dev/flintlock/core/ports/context"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
	"github.com/liquidmetal-dev/flintlock/pkg/planner"
)

const (
	testVMIDString = "namespace/vmid/" + testUID
	testStateDir   = "/tmp/path/to/vm"
	testVMDir      = testStateDir + "/vm/" + testVMIDString
	testRWPath     = testVMDir + "/volumes/root.rw"
	testBasePath   = testVMDir + "/volumes/root.base"
)

// createOverlayTestSpec is createTestSpec with an overlay root, an additional
// volume in full mode and an initrd.
func createOverlayTestSpec(name, ns string) *models.MicroVM {
	vm := createTestSpec(name, ns)
	vm.Spec.RootVolume.Mode = models.VolumeModeOverlay
	vm.Spec.RootVolume.Size = 4096
	vm.Spec.AdditionalVolumes = models.Volumes{
		{
			ID: "data",
			Source: models.VolumeSource{
				Container: &models.ContainerVolumeSource{
					Image: "docker.io/library/data:latest",
				},
			},
		},
	}
	vm.Spec.Initrd = &models.Initrd{
		Image:    "ghcr.io/liquidmetal-dev/flintlock-overlay-init:0.16.0",
		Filename: "initrd.img",
	}

	return vm
}

func testOverlayMounts() *ports.VolumeMounts {
	return &ports.VolumeMounts{
		BaseMount:  models.Mount{Type: models.MountTypeFile, Source: testBasePath},
		RWMount:    models.Mount{Type: models.MountTypeFile, Source: testRWPath},
		BaseDigest: "abc123",
	}
}

func stepNames(steps []planner.Procedure) []string {
	names := make([]string, 0, len(steps))
	for _, step := range steps {
		names = append(names, step.Name())
	}

	return names
}

func TestMicroVMCreateOrUpdatePlan_overlayRoot(t *testing.T) {
	RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	testVM := createOverlayTestSpec("vmid", "namespace")
	mList, mockedPorts := fakePorts(mockCtrl)
	ctx := portsctx.WithPorts(context.Background(), mockedPorts)
	plan := plans.MicroVMCreateOrUpdatePlan(&plans.CreateOrUpdatePlanInput{
		VM:              testVM,
		StateDirectory:  testStateDir,
		SocketDirectory: "/tmp/run",
	})

	mList.MicroVMService.
		EXPECT().
		State(gomock.Any(), gomock.Eq(testVMIDString)).
		Return(ports.MicroVMStatePending, nil).
		AnyTimes()
	mList.MicroVMService.EXPECT().Capabilities().Return(models.Capabilities{}).AnyTimes()
	mList.MicroVMService.EXPECT().Create(gomock.Any(), gomock.Any())
	mList.MicroVMService.EXPECT().Start(gomock.Any(), gomock.Any()).Return(nil)

	mList.NetworkService.
		EXPECT().
		IfaceExists(gomock.Any(), &hostDeviceNameMatcher{}).
		Return(false, nil).
		AnyTimes()
	mList.NetworkService.
		EXPECT().
		IfaceCreate(gomock.Any(), gomock.Any()).
		Return(&ports.IfaceDetails{}, nil).
		Times(2)

	// The root volume goes through the volume service, with the lease owner
	// the image service steps use.
	mList.VolumeService.
		EXPECT().
		Create(gomock.Any(), gomock.Eq(ports.VolumeCreateInput{
			VolumeRef: ports.VolumeRef{VMID: testVM.ID, VolumeID: "root"},
			ImageRef:  "docker.io/library/ubuntu:myimage",
			SizeMB:    4096,
			Owner:     testVMIDString,
		})).
		Return(testOverlayMounts(), nil).
		Times(1)

	// The additional volume, the kernel and the initrd keep the image service.
	for _, spec := range []*ports.ImageMountSpec{
		{
			ImageName:    "docker.io/library/data:latest",
			Owner:        testVMIDString,
			OwnerUsageID: "data",
			Use:          models.ImageUseVolume,
		},
		{
			ImageName:    "docker.io/linuxkit/kernel:5.4.129",
			Owner:        testVMIDString,
			OwnerUsageID: "kernel",
			Use:          models.ImageUseKernel,
		},
		{
			ImageName:    "ghcr.io/liquidmetal-dev/flintlock-overlay-init:0.16.0",
			Owner:        testVMIDString,
			OwnerUsageID: "initrd",
			Use:          models.ImageUseInitrd,
		},
	} {
		mList.ImageService.
			EXPECT().
			PullAndMount(gomock.Any(), gomock.Eq(spec)).
			Return([]models.Mount{{Type: models.MountTypeHostPath}}, nil).
			Times(1)
	}

	steps, createErr := plan.Create(ctx)

	Expect(createErr).NotTo(HaveOccurred())
	Expect(stepNames(steps)).To(Equal([]string{
		"io_create_dir",
		"io_create_dir",
		"runtime_overlay_volume_mount",
		"runtime_volume_mount",
		"runtime_kernel_mount",
		"runtime_initrd_mount",
		"network_iface_create",
		"network_iface_create",
		"microvm_create",
		"microvm_start",
	}))

	for _, step := range steps {
		should, err := step.ShouldDo(ctx)

		Expect(err).NotTo(HaveOccurred())
		Expect(should).To(BeTrue(), step.Name())

		extraSteps, err := step.Do(ctx)

		Expect(err).NotTo(HaveOccurred())
		Expect(extraSteps).To(BeNil())
	}

	Expect(testVM.Status.Volumes["root"]).To(Equal(&models.VolumeStatus{
		Mount:      models.Mount{Type: models.MountTypeFile, Source: testRWPath},
		BaseMount:  &models.Mount{Type: models.MountTypeFile, Source: testBasePath},
		BaseDigest: "abc123",
	}))
}

func TestMicroVMCreateOrUpdatePlan_overlayRootProvisioned(t *testing.T) {
	testCases := []struct {
		name     string
		files    []string
		expected bool
	}{
		{
			name:     "both files exist, the volume is left alone",
			files:    []string{testRWPath, testBasePath},
			expected: false,
		},
		{
			name:     "the writable disk is missing, the volume is provisioned again",
			files:    []string{testBasePath},
			expected: true,
		},
		{
			name:     "the base link is missing, the volume is provisioned again",
			files:    []string{testRWPath},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			testVM := createOverlayTestSpec("vmid", "namespace")
			mounts := testOverlayMounts()
			testVM.Status.Volumes = models.VolumeStatuses{
				"root": {
					Mount:      mounts.RWMount,
					BaseMount:  &mounts.BaseMount,
					BaseDigest: mounts.BaseDigest,
				},
			}

			mList, mockedPorts := fakePorts(mockCtrl)
			for _, file := range tc.files {
				Expect(afero.WriteFile(mockedPorts.FileSystem, file, []byte("x"), 0o644)).To(Succeed())
			}

			ctx := portsctx.WithPorts(context.Background(), mockedPorts)
			plan := plans.MicroVMCreateOrUpdatePlan(&plans.CreateOrUpdatePlanInput{
				VM:              testVM,
				StateDirectory:  testStateDir,
				SocketDirectory: "/tmp/run",
			})

			mList.MicroVMService.
				EXPECT().
				State(gomock.Any(), gomock.Any()).
				Return(ports.MicroVMStatePending, nil).
				AnyTimes()
			mList.MicroVMService.EXPECT().Capabilities().Return(models.Capabilities{}).AnyTimes()
			mList.NetworkService.
				EXPECT().
				IfaceExists(gomock.Any(), gomock.Any()).
				Return(false, nil).
				AnyTimes()

			steps, createErr := plan.Create(ctx)

			Expect(createErr).NotTo(HaveOccurred())

			if tc.expected {
				Expect(stepNames(steps)).To(ContainElement("runtime_overlay_volume_mount"))
			} else {
				Expect(stepNames(steps)).NotTo(ContainElement("runtime_overlay_volume_mount"))
			}
		})
	}
}

func TestMicroVMCreateOrUpdatePlan_overlayRootNoVolumeService(t *testing.T) {
	RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	testVM := createOverlayTestSpec("vmid", "namespace")
	mList, mockedPorts := fakePorts(mockCtrl)
	mockedPorts.VolumeService = nil
	ctx := portsctx.WithPorts(context.Background(), mockedPorts)
	plan := plans.MicroVMCreateOrUpdatePlan(&plans.CreateOrUpdatePlanInput{
		VM:              testVM,
		StateDirectory:  testStateDir,
		SocketDirectory: "/tmp/run",
	})

	mList.MicroVMService.EXPECT().Capabilities().Return(models.Capabilities{}).AnyTimes()

	_, createErr := plan.Create(ctx)

	Expect(createErr).To(MatchError(ContainSubstring("volume service is not available")))
}

func TestMicroVMDeletePlan_overlayRoot(t *testing.T) {
	RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mList, mockedPorts := fakePorts(mockCtrl)
	ctx := portsctx.WithPorts(context.Background(), mockedPorts)
	spec := createOverlayTestSpec("vmid", "namespace")
	spec.Spec.DeletedAt = 1
	// A microvm whose create failed early has no volume status.
	spec.Status.Volumes = nil
	plan := plans.MicroVMDeletePlan(&plans.DeletePlanInput{
		VM:              spec,
		StateDirectory:  testStateDir,
		SocketDirectory: "/tmp/run",
	})

	Expect(mockedPorts.FileSystem.MkdirAll(testVMDir+"/volumes", os.ModeDir)).To(Succeed())
	Expect(mockedPorts.FileSystem.MkdirAll("/tmp/run/"+testUID, os.ModeDir)).To(Succeed())

	mList.MicroVMService.
		EXPECT().
		State(gomock.Any(), gomock.Eq(testVMIDString)).
		Return(ports.MicroVMStateRunning, nil).
		AnyTimes()
	mList.MicroVMService.EXPECT().Capabilities().Return(models.Capabilities{}).AnyTimes()
	mList.MicroVMService.EXPECT().Delete(gomock.Any(), gomock.Eq(testVMIDString)).Return(nil).Times(1)

	mList.MicroVMRepository.EXPECT().Exists(gomock.Any(), gomock.Eq(spec.ID)).Return(true, nil).AnyTimes()
	mList.MicroVMRepository.EXPECT().ReleaseLease(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	mList.NetworkService.
		EXPECT().
		IfaceExists(gomock.Any(), &hostDeviceNameMatcher{}).
		Return(true, nil).
		AnyTimes()
	mList.NetworkService.EXPECT().IfaceDelete(gomock.Any(), &deleteInterfaceMatcher{}).Times(1)

	mList.VolumeService.
		EXPECT().
		Release(gomock.Any(), gomock.Eq(ports.VolumeRef{VMID: spec.ID, VolumeID: "root"})).
		Return(nil).
		Times(1)

	mList.EventService.
		EXPECT().
		Publish(gomock.Any(), gomock.Eq(defaults.TopicMicroVMEvents), gomock.Any()).
		Return(nil).
		AnyTimes()

	steps, createErr := plan.Create(ctx)

	Expect(createErr).NotTo(HaveOccurred())
	Expect(stepNames(steps)).To(Equal([]string{
		"microvm_delete",
		"runtime_repo_release",
		"network_iface_delete",
		"runtime_volume_release",
		"io_delete_dir",
		"io_delete_dir",
		"event_publish",
	}))

	for _, step := range steps {
		should, err := step.ShouldDo(ctx)

		Expect(err).NotTo(HaveOccurred())
		Expect(should).To(BeTrue(), step.Name())

		extraSteps, err := step.Do(ctx)

		Expect(err).NotTo(HaveOccurred())
		Expect(extraSteps).To(BeNil())
	}

	// The directory is gone, so a second pass has no release step.
	mList.NetworkService.
		EXPECT().
		IfaceDelete(gomock.Any(), &deleteInterfaceMatcher{}).
		AnyTimes()

	steps, createErr = plan.Create(ctx)

	Expect(createErr).NotTo(HaveOccurred())
	Expect(stepNames(steps)).NotTo(ContainElement("runtime_volume_release"))
}

func TestMicroVMDeletePlan_fullRootHasNoRelease(t *testing.T) {
	RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mList, mockedPorts := fakePorts(mockCtrl)
	mockedPorts.VolumeService = nil
	ctx := portsctx.WithPorts(context.Background(), mockedPorts)
	spec := createTestSpec("vmid", "namespace")
	spec.Spec.DeletedAt = 1
	plan := plans.MicroVMDeletePlan(&plans.DeletePlanInput{
		VM:              spec,
		StateDirectory:  testStateDir,
		SocketDirectory: "/tmp/run",
	})

	Expect(mockedPorts.FileSystem.MkdirAll(testVMDir, os.ModeDir)).To(Succeed())

	mList.MicroVMService.
		EXPECT().
		State(gomock.Any(), gomock.Any()).
		Return(ports.MicroVMStateRunning, nil).
		AnyTimes()
	mList.MicroVMService.EXPECT().Capabilities().Return(models.Capabilities{}).AnyTimes()
	mList.MicroVMRepository.EXPECT().Exists(gomock.Any(), gomock.Any()).Return(true, nil).AnyTimes()
	mList.NetworkService.EXPECT().IfaceExists(gomock.Any(), gomock.Any()).Return(true, nil).AnyTimes()

	steps, createErr := plan.Create(ctx)

	Expect(createErr).NotTo(HaveOccurred())
	Expect(stepNames(steps)).NotTo(ContainElement("runtime_volume_release"))
}
