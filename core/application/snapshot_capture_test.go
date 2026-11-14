package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"

	"github.com/liquidmetal-dev/flintlock/core/application"
	coreerrs "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/infrastructure/mock"
)

const captureTestDir = "/snapshots/" + testSnapshotUID

var errCaptureTest = errors.New("vmm refused the snapshot")

type captureAppMocks struct {
	repo      *mock.MockMicroVMRepository
	snapshots *mock.MockSnapshotRepository
	provider  *mock.MockMicroVMService
	volumes   *mock.MockVolumeService
	hostInfo  *mock.MockHostInfoService
	ids       *mock.MockIDService
}

func newCaptureTestApp(t *testing.T, cfg *application.Config) (application.App, captureAppMocks) {
	t.Helper()

	ctrl := gomock.NewController(t)

	m := captureAppMocks{
		repo:      mock.NewMockMicroVMRepository(ctrl),
		snapshots: mock.NewMockSnapshotRepository(ctrl),
		provider:  mock.NewMockMicroVMService(ctrl),
		volumes:   mock.NewMockVolumeService(ctrl),
		hostInfo:  mock.NewMockHostInfoService(ctrl),
		ids:       mock.NewMockIDService(ctrl),
	}

	m.provider.EXPECT().Capabilities().Return(snapshotCapabilities).AnyTimes()
	m.snapshots.EXPECT().Dir(testSnapshotUID).Return(captureTestDir).AnyTimes()
	m.ids.EXPECT().GenerateRandom().Return("exec1", nil).AnyTimes()

	app := application.New(cfg, &ports.Collection{
		Repo:              m.repo,
		SnapshotRepo:      m.snapshots,
		MicrovmProviders:  map[string]ports.MicroVMService{"mock": m.provider},
		VolumeService:     m.volumes,
		HostInfoService:   m.hostInfo,
		IdentifierService: m.ids,
		FileSystem:        afero.NewMemMapFs(),
		Clock:             func() time.Time { return snapshotTestTime },
	})

	return app, m
}

func captureRecord(vm *models.MicroVM) *models.Snapshot {
	return &models.Snapshot{
		UID:       testSnapshotUID,
		Name:      "snap1",
		Namespace: vm.ID.Namespace(),
		Source:    vm.ID,
		Status:    models.SnapshotStatus{Phase: models.SnapshotPhaseCapturing},
	}
}

func TestApp_CaptureSnapshot(t *testing.T) {
	RegisterTestingT(t)

	app, m := newCaptureTestApp(t, &application.Config{})
	vm := snapshotSourceVM()
	record := captureRecord(vm)

	m.repo.EXPECT().Get(gomock.Any(), ports.RepositoryGetOptions{UID: testUID}).Return(vm, nil)
	m.provider.EXPECT().Drives(gomock.Any()).Return(models.Drives{{ID: "rw", Role: models.DriveRoleRootRW}}, nil)

	gomock.InOrder(
		m.provider.EXPECT().State(gomock.Any(), vm.ID.String()).Return(ports.MicroVMStateRunning, nil),
		m.provider.EXPECT().Pause(gomock.Any(), vm).Return(nil),
		m.provider.EXPECT().Snapshot(gomock.Any(), vm, captureTestDir).Return(nil),
		m.volumes.EXPECT().Capture(gomock.Any(), ports.VolumeRef{VMID: vm.ID, VolumeID: "root"}, captureTestDir).
			Return(&models.CapturedVolume{BaseDigest: "abc"}, nil),
		m.provider.EXPECT().Resume(gomock.Any(), vm).Return(nil),
		m.hostInfo.EXPECT().Facts(gomock.Any(), "mock").Return(&models.HostFacts{}, nil),
	)

	Expect(app.CaptureSnapshot(context.Background(), record)).To(Succeed())
	Expect(record.Status.Phase).To(Equal(models.SnapshotPhaseCapturing), "the phase is the caller's to set")
}

func TestApp_CaptureSnapshot_FailureResumesAndCleansUp(t *testing.T) {
	RegisterTestingT(t)

	app, m := newCaptureTestApp(t, &application.Config{})
	vm := snapshotSourceVM()

	m.repo.EXPECT().Get(gomock.Any(), gomock.Any()).Return(vm, nil)

	gomock.InOrder(
		m.provider.EXPECT().State(gomock.Any(), vm.ID.String()).Return(ports.MicroVMStateRunning, nil),
		m.provider.EXPECT().Pause(gomock.Any(), vm).Return(nil),
		m.provider.EXPECT().Snapshot(gomock.Any(), vm, captureTestDir).Return(errCaptureTest),
		m.provider.EXPECT().Resume(gomock.Any(), vm).Return(nil),
		m.snapshots.EXPECT().RemoveFiles(gomock.Any(), testSnapshotUID).Return(nil),
	)

	Expect(app.CaptureSnapshot(context.Background(), captureRecord(vm))).To(MatchError(errCaptureTest))
}

func TestApp_CaptureSnapshot_PauseBudgetFromConfig(t *testing.T) {
	RegisterTestingT(t)

	app, m := newCaptureTestApp(t, &application.Config{SnapshotMaxPause: 30 * time.Millisecond})
	vm := snapshotSourceVM()

	m.repo.EXPECT().Get(gomock.Any(), gomock.Any()).Return(vm, nil)
	m.provider.EXPECT().State(gomock.Any(), gomock.Any()).Return(ports.MicroVMStateRunning, nil)
	m.provider.EXPECT().Pause(gomock.Any(), vm).Return(nil)
	m.provider.EXPECT().Snapshot(gomock.Any(), vm, captureTestDir).DoAndReturn(
		func(ctx context.Context, _ *models.MicroVM, _ string) error {
			<-ctx.Done()

			return ctx.Err()
		})
	m.provider.EXPECT().Resume(gomock.Any(), vm).Return(nil)
	m.snapshots.EXPECT().RemoveFiles(gomock.Any(), testSnapshotUID).Return(nil)

	err := app.CaptureSnapshot(context.Background(), captureRecord(vm))

	Expect(err).To(MatchError(ContainSubstring("pause budget of 30ms exceeded")))
}

func TestApp_CaptureSnapshot_SourceNotFound(t *testing.T) {
	RegisterTestingT(t)

	app, m := newCaptureTestApp(t, &application.Config{})
	vm := snapshotSourceVM()

	m.repo.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, nil)

	err := app.CaptureSnapshot(context.Background(), captureRecord(vm))

	Expect(coreerrs.IsSpecNotFound(err)).To(BeTrue())
}

func TestApp_CaptureSnapshot_SourceGetFails(t *testing.T) {
	RegisterTestingT(t)

	app, m := newCaptureTestApp(t, &application.Config{})
	vm := snapshotSourceVM()

	m.repo.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, errCaptureTest)

	Expect(app.CaptureSnapshot(context.Background(), captureRecord(vm))).To(MatchError(errCaptureTest))
}
