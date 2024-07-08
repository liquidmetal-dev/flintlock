package runtime_test

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	internalerr "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/core/steps/runtime"
	"github.com/liquidmetal-dev/flintlock/infrastructure/mock"
	g "github.com/onsi/gomega"
)

func testVMWithMount() *models.MicroVM {
	vmid, _ := models.NewVMID("vm", "ns", "uid")
	return &models.MicroVM{
		ID:      *vmid,
		Version: 1,
		Status: models.MicroVMStatus{
			Volumes: models.VolumeStatuses{},
		},
		Spec: models.MicroVMSpec{
			RootVolume: models.Volume{
				ID:         "rootVolume",
				IsReadOnly: true,
				Source: models.VolumeSource{
					Container: &models.ContainerVolumeSource{
						Image: "myimage:tag",
					},
				},
				Size: 20,
			},
			AdditionalVolumes: models.Volumes{
				models.Volume{
					ID:         "homeVolume",
					IsReadOnly: false,
					Source: models.VolumeSource{
						Container: &models.ContainerVolumeSource{
							Image: "myhomeimage:tag",
						},
					},
					Size: 50,
				},
			},
			VCPU:       1,
			MemoryInMb: 512,
		},
	}
}

func testMount(source string) models.Mount {
	return models.Mount{
		Type:   "mounttype",
		Source: source,
	}
}

func testVolumeMountSpec(vmid *models.VMID, volume *models.Volume) *ports.ImageMountSpec {
	return &ports.ImageMountSpec{
		ImageName:    string(volume.Source.Container.Image),
		Owner:        vmid.String(),
		OwnerUsageID: volume.ID,
		Use:          models.ImageUseVolume,
	}
}

func TestVolumeMount(t *testing.T) {
	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	imageService := mock.NewMockImageService(mockCtrl)
	ctx := context.Background()
	vm := testVMWithMount()

	expectedVolume := testMount("firstsouArce")

	for _, volume := range vm.Spec.AdditionalVolumes {
		vm.Status.Volumes[volume.ID] = &models.VolumeStatus{}
		step := runtime.NewVolumeMount(
			&vm.ID,
			&volume,
			vm.Status.Volumes[volume.ID],
			imageService,
		)

		imageService.
			EXPECT().
			PullAndMount(ctx, testVolumeMountSpec(&vm.ID, &volume)).
			Return([]models.Mount{expectedVolume}, nil)

		shouldDo, shouldErr := step.ShouldDo(ctx)
		subSteps, doErr := step.Do(ctx)

		g.Expect(shouldDo).To(g.BeTrue())
		g.Expect(shouldErr).To(g.BeNil())
		g.Expect(subSteps).To(g.BeEmpty())
		g.Expect(doErr).To(g.BeNil())

		verifyErr := step.Verify(ctx)
		g.Expect(verifyErr).To(g.BeNil())
	}

	g.Expect(vm.Status.Volumes).To(g.HaveLen(1))
}

func TestVolumeMount_statusAlreadySetBoth(t *testing.T) {
	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	imageService := mock.NewMockImageService(mockCtrl)
	ctx := context.Background()
	vm := testVMWithMount()

	// RootVolume

	for _, volume := range vm.Spec.AdditionalVolumes {
		expectedMount := testMount("firstsouArce")
		mountSpec := testVolumeMountSpec(&vm.ID, &volume)

		vm.Status.Volumes[volume.ID] = &models.VolumeStatus{
			Mount: expectedMount,
		}
		step := runtime.NewVolumeMount(
			&vm.ID,
			&volume,
			vm.Status.Volumes[volume.ID],
			imageService,
		)

		imageService.
			EXPECT().
			IsMounted(ctx, mountSpec).
			Return(true, nil)

		imageService.
			EXPECT().
			PullAndMount(ctx, testVolumeMountSpec(&vm.ID, &volume)).
			Return([]models.Mount{expectedMount}, nil)

		shouldDo, shouldErr := step.ShouldDo(ctx)
		subSteps, doErr := step.Do(ctx)

		g.Expect(shouldDo).To(g.BeFalse())
		g.Expect(shouldErr).To(g.BeNil())
		g.Expect(subSteps).To(g.BeEmpty())
		g.Expect(doErr).To(g.BeNil())

		verifyErr := step.Verify(ctx)
		g.Expect(verifyErr).To(g.BeNil())
	}

	g.Expect(vm.Status.Volumes).To(g.HaveLen(1))
}

func TestVolumeMount_retry(t *testing.T) {
	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	imageService := mock.NewMockImageService(mockCtrl)
	ctx := context.Background()
	vm := testVMWithMount()

	for _, volume := range vm.Spec.AdditionalVolumes {
		expectedMount := testMount("firstsouArce")
		mountSpec := testVolumeMountSpec(&vm.ID, &volume)

		vm.Status.Volumes[volume.ID] = &models.VolumeStatus{
			Mount: expectedMount,
		}
		step := runtime.NewVolumeMount(
			&vm.ID,
			&volume,
			vm.Status.Volumes[volume.ID],
			imageService,
		)

		imageService.
			EXPECT().
			IsMounted(ctx, mountSpec).
			Return(true, nil).
			Times(2)

		imageService.
			EXPECT().
			PullAndMount(ctx, testVolumeMountSpec(&vm.ID, &volume)).
			Return([]models.Mount{expectedMount}, nil).
			Times(2)

		shouldDo, shouldErr := step.ShouldDo(ctx)
		subSteps, doErr := step.Do(ctx)

		g.Expect(shouldDo).To(g.BeFalse())
		g.Expect(shouldErr).To(g.BeNil())
		g.Expect(subSteps).To(g.BeEmpty())
		g.Expect(doErr).To(g.BeNil())

		shouldDo, shouldErr = step.ShouldDo(ctx)
		subSteps, doErr = step.Do(ctx)

		g.Expect(shouldDo).To(g.BeFalse())
		g.Expect(shouldErr).To(g.BeNil())
		g.Expect(subSteps).To(g.BeEmpty())
		g.Expect(doErr).To(g.BeNil())

		verifyErr := step.Verify(ctx)
		g.Expect(verifyErr).To(g.BeNil())
	}

	g.Expect(vm.Status.Volumes).To(g.HaveLen(1))
}

func TestVolumeMount_IsMountedError(t *testing.T) {
	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	imageService := mock.NewMockImageService(mockCtrl)
	ctx := context.Background()
	vm := testVMWithMount()

	volume := vm.Spec.AdditionalVolumes[0]
	mountSpec := testVolumeMountSpec(&vm.ID, &volume)
	expectedMount := testMount("firstsouArce")
	vm.Status.Volumes[volume.ID] = &models.VolumeStatus{
		Mount: expectedMount,
	}
	step := runtime.NewVolumeMount(
		&vm.ID,
		&volume,
		vm.Status.Volumes[volume.ID],
		imageService,
	)

	imageService.
		EXPECT().
		IsMounted(ctx, mountSpec).
		Return(false, errors.New("nope"))

	shouldDo, shouldErr := step.ShouldDo(ctx)

	g.Expect(shouldDo).To(g.BeFalse())
	g.Expect(shouldErr).ToNot(g.BeNil())
}

func TestVolumeMount_doError(t *testing.T) {
	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	imageService := mock.NewMockImageService(mockCtrl)
	ctx := context.Background()
	vm := testVMWithMount()

	volume := vm.Spec.AdditionalVolumes[0]
	expectedMount := testMount("firstsouArce")
	vm.Status.Volumes[volume.ID] = &models.VolumeStatus{
		Mount: expectedMount,
	}
	step := runtime.NewVolumeMount(
		&vm.ID,
		&volume,
		vm.Status.Volumes[volume.ID],
		imageService,
	)

	// PullAndMount error.
	imageService.
		EXPECT().
		PullAndMount(ctx, testVolumeMountSpec(&vm.ID, &volume)).
		Return([]models.Mount{}, errors.New("pull error"))

	extraSteps, doErr := step.Do(ctx)

	g.Expect(extraSteps).To(g.BeEmpty())
	g.Expect(doErr).ToNot(g.BeNil())

	// Empty reponse from PullAndMount.
	imageService.
		EXPECT().
		PullAndMount(ctx, testVolumeMountSpec(&vm.ID, &volume)).
		Return([]models.Mount{}, nil)

	extraSteps, doErr = step.Do(ctx)

	g.Expect(extraSteps).To(g.BeEmpty())
	g.Expect(doErr).To(g.MatchError(internalerr.ErrNoVolumeMount))

	verifyErr := step.Verify(ctx)
	g.Expect(verifyErr).To(g.BeNil())
}

func TestVolumeMount_nilStatus(t *testing.T) {
	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	imageService := mock.NewMockImageService(mockCtrl)
	ctx := context.Background()
	vm := testVMWithMount()

	var volumeStatus *models.VolumeStatus

	volume := vm.Spec.AdditionalVolumes[0]
	vm.Status.Volumes[volume.ID] = volumeStatus
	step := runtime.NewVolumeMount(
		&vm.ID,
		&volume,
		vm.Status.Volumes[volume.ID],
		imageService,
	)

	extraSteps, doErr := step.Do(ctx)

	g.Expect(extraSteps).To(g.BeEmpty())
	g.Expect(doErr).To(g.MatchError(internalerr.ErrMissingStatusInfo))

	verifyErr := step.Verify(ctx)
	g.Expect(verifyErr).To(g.BeNil())
}
