package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/core/application"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
)

const restartedDuringCapture = "flintlockd restarted during the capture"

func recordInPhase(vm *models.MicroVM, phase models.SnapshotPhase) *models.Snapshot {
	record := captureRecord(vm)
	record.Status.Phase = phase

	return record
}

func expectFailedOnRestart(m captureAppMocks, vm *models.MicroVM) []*gomock.Call {
	calls := expectPhase(m, vm, models.SnapshotPhaseFailed, restartedDuringCapture)

	return append(calls, m.snapshots.EXPECT().RemoveFiles(gomock.Any(), testSnapshotUID).Return(nil))
}

func TestApp_RecoverSnapshots_FailsCleansAndResumes(t *testing.T) {
	for _, phase := range []models.SnapshotPhase{models.SnapshotPhasePending, models.SnapshotPhaseCapturing} {
		t.Run(string(phase), func(t *testing.T) {
			RegisterTestingT(t)

			app, m := newCaptureTestApp(t, &application.Config{})
			vm := snapshotSourceVM()

			m.snapshots.EXPECT().List(gomock.Any(), ports.SnapshotListOptions{}).
				Return([]*models.Snapshot{recordInPhase(vm, phase)}, nil)

			calls := []*gomock.Call{
				m.repo.EXPECT().Get(gomock.Any(), ports.RepositoryGetOptions{UID: testUID}).Return(vm, nil),
				m.provider.EXPECT().State(gomock.Any(), vm.ID.String()).Return(ports.MicroVMStatePaused, nil),
				m.provider.EXPECT().Resume(gomock.Any(), vm).Return(nil),
			}
			gomock.InOrder(append(calls, expectFailedOnRestart(m, vm)...)...)

			Expect(app.RecoverSnapshots(context.Background())).To(Succeed())
		})
	}
}

func TestApp_RecoverSnapshots_RunningSourceIsNotResumed(t *testing.T) {
	RegisterTestingT(t)

	app, m := newCaptureTestApp(t, &application.Config{})
	vm := snapshotSourceVM()

	m.snapshots.EXPECT().List(gomock.Any(), gomock.Any()).
		Return([]*models.Snapshot{recordInPhase(vm, models.SnapshotPhaseCapturing)}, nil)
	m.repo.EXPECT().Get(gomock.Any(), gomock.Any()).Return(vm, nil)
	m.provider.EXPECT().State(gomock.Any(), vm.ID.String()).Return(ports.MicroVMStateRunning, nil)
	gomock.InOrder(expectFailedOnRestart(m, vm)...)

	Expect(app.RecoverSnapshots(context.Background())).To(Succeed())
}

func TestApp_RecoverSnapshots_SourceThatWantsPausedIsNotResumed(t *testing.T) {
	RegisterTestingT(t)

	app, m := newCaptureTestApp(t, &application.Config{})
	vm := snapshotSourceVM()
	vm.Spec.PowerState = models.PowerStatePaused

	m.snapshots.EXPECT().List(gomock.Any(), gomock.Any()).
		Return([]*models.Snapshot{recordInPhase(vm, models.SnapshotPhaseCapturing)}, nil)
	m.repo.EXPECT().Get(gomock.Any(), gomock.Any()).Return(vm, nil)
	gomock.InOrder(expectFailedOnRestart(m, vm)...)

	Expect(app.RecoverSnapshots(context.Background())).To(Succeed())
}

func TestApp_RecoverSnapshots_SourceGone(t *testing.T) {
	RegisterTestingT(t)

	app, m := newCaptureTestApp(t, &application.Config{})
	vm := snapshotSourceVM()

	m.snapshots.EXPECT().List(gomock.Any(), gomock.Any()).
		Return([]*models.Snapshot{recordInPhase(vm, models.SnapshotPhaseCapturing)}, nil)
	m.repo.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, nil)
	gomock.InOrder(expectFailedOnRestart(m, vm)...)

	Expect(app.RecoverSnapshots(context.Background())).To(Succeed())
}

func TestApp_RecoverSnapshots_FinishedSnapshotsAreLeftAlone(t *testing.T) {
	RegisterTestingT(t)

	app, m := newCaptureTestApp(t, &application.Config{})
	vm := snapshotSourceVM()

	m.snapshots.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*models.Snapshot{
		recordInPhase(vm, models.SnapshotPhaseReady),
		recordInPhase(vm, models.SnapshotPhaseFailed),
	}, nil)

	Expect(app.RecoverSnapshots(context.Background())).To(Succeed())
}

func TestApp_RecoverSnapshots_ErrorsDoNotStopTheRecovery(t *testing.T) {
	RegisterTestingT(t)

	app, m := newCaptureTestApp(t, &application.Config{})
	vm := snapshotSourceVM()
	resumeErr := errors.New("vmm gone")
	removeErr := errors.New("busy")

	m.snapshots.EXPECT().List(gomock.Any(), gomock.Any()).
		Return([]*models.Snapshot{recordInPhase(vm, models.SnapshotPhaseCapturing)}, nil)
	m.repo.EXPECT().Get(gomock.Any(), gomock.Any()).Return(vm, nil)
	m.provider.EXPECT().State(gomock.Any(), gomock.Any()).Return(ports.MicroVMStatePaused, nil)
	m.provider.EXPECT().Resume(gomock.Any(), vm).Return(resumeErr)
	m.snapshots.EXPECT().Update(gomock.Any(), snapshotPhaseMatcher{
		phase: models.SnapshotPhaseFailed, err: restartedDuringCapture,
	}).Return(nil)
	m.events.EXPECT().Publish(gomock.Any(), defaults.TopicSnapshotEvents, gomock.Any()).Return(nil)
	m.snapshots.EXPECT().RemoveFiles(gomock.Any(), testSnapshotUID).Return(removeErr)

	err := app.RecoverSnapshots(context.Background())

	Expect(err).To(MatchError(resumeErr))
	Expect(err).To(MatchError(removeErr))
}

func TestApp_RecoverSnapshots_ListFails(t *testing.T) {
	RegisterTestingT(t)

	app, m := newCaptureTestApp(t, &application.Config{})

	m.snapshots.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, errCaptureTest)

	Expect(app.RecoverSnapshots(context.Background())).To(MatchError(errCaptureTest))
}
