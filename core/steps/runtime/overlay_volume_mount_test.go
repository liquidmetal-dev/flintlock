package runtime_test

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	g "github.com/onsi/gomega"
	"github.com/spf13/afero"

	internalerr "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/core/steps/runtime"
	"github.com/liquidmetal-dev/flintlock/infrastructure/mock"
)

const (
	testOverlayRWPath   = "/var/lib/flintlock/vm/ns/vm/uid/volumes/root.rw"
	testOverlayBasePath = "/var/lib/flintlock/vm/ns/vm/uid/volumes/root.base"
)

func testVMWithOverlayRoot() *models.MicroVM {
	vmid, _ := models.NewVMID("vm", "ns", "uid")

	return &models.MicroVM{
		ID:      *vmid,
		Version: 1,
		Status: models.MicroVMStatus{
			Volumes: models.VolumeStatuses{},
		},
		Spec: models.MicroVMSpec{
			RootVolume: models.Volume{
				ID:   "rootVolume",
				Mode: models.VolumeModeOverlay,
				Source: models.VolumeSource{
					Container: &models.ContainerVolumeSource{
						Image: "myimage:tag",
					},
				},
				Size: 2048,
			},
			VCPU:       1,
			MemoryInMb: 512,
		},
	}
}

func testOverlayCreateInput(vm *models.MicroVM) ports.VolumeCreateInput {
	return ports.VolumeCreateInput{
		VolumeRef: ports.VolumeRef{
			VMID:     vm.ID,
			VolumeID: vm.Spec.RootVolume.ID,
		},
		ImageRef: "myimage:tag",
		SizeMB:   2048,
		Owner:    vm.ID.String(),
	}
}

func testOverlayMounts() *ports.VolumeMounts {
	return &ports.VolumeMounts{
		BaseMount:  models.Mount{Type: models.MountTypeFile, Source: testOverlayBasePath},
		RWMount:    models.Mount{Type: models.MountTypeFile, Source: testOverlayRWPath},
		BaseDigest: "abc123",
	}
}

func testOverlayStatus() *models.VolumeStatus {
	mounts := testOverlayMounts()

	return &models.VolumeStatus{
		Mount:      mounts.RWMount,
		BaseMount:  &mounts.BaseMount,
		BaseDigest: mounts.BaseDigest,
	}
}

func TestOverlayVolumeMount_Do(t *testing.T) {
	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	volumeService := mock.NewMockVolumeService(mockCtrl)
	ctx := context.Background()
	vm := testVMWithOverlayRoot()
	status := &models.VolumeStatus{}
	vm.Status.Volumes[vm.Spec.RootVolume.ID] = status

	step := runtime.NewOverlayVolumeMount(&vm.ID, &vm.Spec.RootVolume, status, volumeService, afero.NewMemMapFs())

	volumeService.
		EXPECT().
		Create(ctx, testOverlayCreateInput(vm)).
		Return(testOverlayMounts(), nil)

	shouldDo, shouldErr := step.ShouldDo(ctx)
	subSteps, doErr := step.Do(ctx)

	g.Expect(shouldDo).To(g.BeTrue())
	g.Expect(shouldErr).To(g.BeNil())
	g.Expect(subSteps).To(g.BeEmpty())
	g.Expect(doErr).To(g.BeNil())
	g.Expect(step.Verify(ctx)).To(g.BeNil())

	g.Expect(status).To(g.Equal(testOverlayStatus()))
}

func TestOverlayVolumeMount_ShouldDo(t *testing.T) {
	testCases := []struct {
		name     string
		status   *models.VolumeStatus
		files    []string
		expected bool
	}{
		{
			name:     "nil status",
			status:   nil,
			files:    []string{testOverlayRWPath, testOverlayBasePath},
			expected: true,
		},
		{
			name:     "status without mounts",
			status:   &models.VolumeStatus{},
			files:    []string{testOverlayRWPath, testOverlayBasePath},
			expected: true,
		},
		{
			name: "status with the writable mount only",
			status: &models.VolumeStatus{
				Mount: models.Mount{Type: models.MountTypeFile, Source: testOverlayRWPath},
			},
			files:    []string{testOverlayRWPath, testOverlayBasePath},
			expected: true,
		},
		{
			name:     "status with both mounts and no files",
			status:   testOverlayStatus(),
			files:    nil,
			expected: true,
		},
		{
			name:     "status with both mounts and the base missing",
			status:   testOverlayStatus(),
			files:    []string{testOverlayRWPath},
			expected: true,
		},
		{
			name:     "status with both mounts and the writable disk missing",
			status:   testOverlayStatus(),
			files:    []string{testOverlayBasePath},
			expected: true,
		},
		{
			name:     "status with both mounts and both files",
			status:   testOverlayStatus(),
			files:    []string{testOverlayRWPath, testOverlayBasePath},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g.RegisterTestingT(t)
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			volumeService := mock.NewMockVolumeService(mockCtrl)
			ctx := context.Background()
			vm := testVMWithOverlayRoot()
			fs := afero.NewMemMapFs()

			for _, file := range tc.files {
				g.Expect(afero.WriteFile(fs, file, []byte("x"), 0o644)).To(g.Succeed())
			}

			step := runtime.NewOverlayVolumeMount(&vm.ID, &vm.Spec.RootVolume, tc.status, volumeService, fs)

			shouldDo, err := step.ShouldDo(ctx)

			g.Expect(err).NotTo(g.HaveOccurred())
			g.Expect(shouldDo).To(g.Equal(tc.expected))
		})
	}
}

func TestOverlayVolumeMount_doError(t *testing.T) {
	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	volumeService := mock.NewMockVolumeService(mockCtrl)
	ctx := context.Background()
	vm := testVMWithOverlayRoot()
	status := &models.VolumeStatus{}

	step := runtime.NewOverlayVolumeMount(&vm.ID, &vm.Spec.RootVolume, status, volumeService, afero.NewMemMapFs())

	volumeService.
		EXPECT().
		Create(ctx, testOverlayCreateInput(vm)).
		Return(nil, errors.New("create error"))

	subSteps, doErr := step.Do(ctx)

	g.Expect(subSteps).To(g.BeEmpty())
	g.Expect(doErr).To(g.MatchError(g.ContainSubstring("create error")))
	g.Expect(status).To(g.Equal(&models.VolumeStatus{}))

	volumeService.
		EXPECT().
		Create(ctx, testOverlayCreateInput(vm)).
		Return(nil, nil)

	subSteps, doErr = step.Do(ctx)

	g.Expect(subSteps).To(g.BeEmpty())
	g.Expect(doErr).To(g.MatchError(internalerr.ErrNoVolumeMount))
}

func TestOverlayVolumeMount_nilStatus(t *testing.T) {
	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	volumeService := mock.NewMockVolumeService(mockCtrl)
	ctx := context.Background()
	vm := testVMWithOverlayRoot()

	step := runtime.NewOverlayVolumeMount(&vm.ID, &vm.Spec.RootVolume, nil, volumeService, afero.NewMemMapFs())

	subSteps, doErr := step.Do(ctx)

	g.Expect(subSteps).To(g.BeEmpty())
	g.Expect(doErr).To(g.MatchError(internalerr.ErrMissingStatusInfo))
}
