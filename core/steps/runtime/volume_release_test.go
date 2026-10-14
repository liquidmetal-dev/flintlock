package runtime_test

import (
	"context"
	"errors"
	"os"
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

const testOverlayVMDir = "/var/lib/flintlock/vm/ns/vm/uid"

func testOverlayReleaseRef(vm *models.MicroVM) ports.VolumeRef {
	return ports.VolumeRef{
		VMID:     vm.ID,
		VolumeID: vm.Spec.RootVolume.ID,
	}
}

func TestVolumeRelease_Do(t *testing.T) {
	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	volumeService := mock.NewMockVolumeService(mockCtrl)
	ctx := context.Background()
	vm := testVMWithOverlayRoot()
	fs := afero.NewMemMapFs()
	g.Expect(fs.MkdirAll(testOverlayVMDir, os.ModeDir)).To(g.Succeed())

	step := runtime.NewVolumeRelease(vm, testOverlayVMDir, volumeService, fs)

	volumeService.
		EXPECT().
		Release(ctx, testOverlayReleaseRef(vm)).
		Return(nil)

	shouldDo, shouldErr := step.ShouldDo(ctx)
	subSteps, doErr := step.Do(ctx)

	g.Expect(shouldDo).To(g.BeTrue())
	g.Expect(shouldErr).To(g.BeNil())
	g.Expect(subSteps).To(g.BeEmpty())
	g.Expect(doErr).To(g.BeNil())
	g.Expect(step.Verify(ctx)).To(g.BeNil())
}

func TestVolumeRelease_ShouldDo(t *testing.T) {
	testCases := []struct {
		name     string
		mode     models.VolumeMode
		noSource bool
		dir      bool
		expected bool
	}{
		{
			name:     "overlay root with a state directory",
			mode:     models.VolumeModeOverlay,
			dir:      true,
			expected: true,
		},
		{
			name:     "overlay root without a state directory",
			mode:     models.VolumeModeOverlay,
			dir:      false,
			expected: false,
		},
		{
			name:     "full root",
			mode:     models.VolumeModeFull,
			dir:      true,
			expected: false,
		},
		{
			name:     "unset mode",
			mode:     "",
			dir:      true,
			expected: false,
		},
		{
			name:     "overlay root with no container source",
			mode:     models.VolumeModeOverlay,
			noSource: true,
			dir:      true,
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
			vm.Spec.RootVolume.Mode = tc.mode

			if tc.noSource {
				vm.Spec.RootVolume.Source.Container = nil
			}

			fs := afero.NewMemMapFs()
			if tc.dir {
				g.Expect(fs.MkdirAll(testOverlayVMDir, os.ModeDir)).To(g.Succeed())
			}

			step := runtime.NewVolumeRelease(vm, testOverlayVMDir, volumeService, fs)

			shouldDo, err := step.ShouldDo(ctx)

			g.Expect(err).NotTo(g.HaveOccurred())
			g.Expect(shouldDo).To(g.Equal(tc.expected))
		})
	}
}

func TestVolumeRelease_emptyStatus(t *testing.T) {
	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	volumeService := mock.NewMockVolumeService(mockCtrl)
	ctx := context.Background()
	vm := testVMWithOverlayRoot()
	vm.Status = models.MicroVMStatus{}
	fs := afero.NewMemMapFs()
	g.Expect(fs.MkdirAll(testOverlayVMDir, os.ModeDir)).To(g.Succeed())

	step := runtime.NewVolumeRelease(vm, testOverlayVMDir, volumeService, fs)

	volumeService.
		EXPECT().
		Release(ctx, testOverlayReleaseRef(vm)).
		Return(nil)

	shouldDo, shouldErr := step.ShouldDo(ctx)
	g.Expect(shouldErr).NotTo(g.HaveOccurred())
	g.Expect(shouldDo).To(g.BeTrue())

	_, doErr := step.Do(ctx)
	g.Expect(doErr).NotTo(g.HaveOccurred())
}

func TestVolumeRelease_doError(t *testing.T) {
	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	volumeService := mock.NewMockVolumeService(mockCtrl)
	ctx := context.Background()
	vm := testVMWithOverlayRoot()

	step := runtime.NewVolumeRelease(vm, testOverlayVMDir, volumeService, afero.NewMemMapFs())

	volumeService.
		EXPECT().
		Release(ctx, testOverlayReleaseRef(vm)).
		Return(errors.New("release error"))

	subSteps, doErr := step.Do(ctx)

	g.Expect(subSteps).To(g.BeEmpty())
	g.Expect(doErr).To(g.MatchError(g.ContainSubstring("release error")))
}

func TestVolumeRelease_nilSpec(t *testing.T) {
	g.RegisterTestingT(t)
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	volumeService := mock.NewMockVolumeService(mockCtrl)
	ctx := context.Background()

	step := runtime.NewVolumeRelease(nil, testOverlayVMDir, volumeService, afero.NewMemMapFs())

	shouldDo, shouldErr := step.ShouldDo(ctx)
	g.Expect(shouldDo).To(g.BeFalse())
	g.Expect(shouldErr).To(g.MatchError(internalerr.ErrSpecRequired))

	subSteps, doErr := step.Do(ctx)
	g.Expect(subSteps).To(g.BeEmpty())
	g.Expect(doErr).To(g.MatchError(internalerr.ErrSpecRequired))
}
