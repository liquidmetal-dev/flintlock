package application_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/api/events"
	"github.com/liquidmetal-dev/flintlock/core/application"
	coreerrs "github.com/liquidmetal-dev/flintlock/core/errors"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
)

// snapshotPhaseMatcher matches a snapshot in a phase, with an error that holds
// err, when the call is made: the app changes the phase of the same record on
// each update.
type snapshotPhaseMatcher struct {
	phase models.SnapshotPhase
	err   string
}

func (m snapshotPhaseMatcher) Matches(x interface{}) bool {
	snapshot, ok := x.(*models.Snapshot)

	return ok && snapshot.Status.Phase == m.phase && hasError(snapshot.Status.Error, m.err)
}

func (m snapshotPhaseMatcher) String() string {
	return "is a snapshot in phase " + string(m.phase) + " with an error that holds " + m.err
}

// snapshotUpdatedMatcher matches the SnapshotUpdated event of the test snapshot
// for a phase, with an error that holds err.
type snapshotUpdatedMatcher struct {
	vm    *models.MicroVM
	phase models.SnapshotPhase
	err   string
}

func (m snapshotUpdatedMatcher) Matches(x interface{}) bool {
	evt, ok := x.(*events.SnapshotUpdated)
	if !ok || !hasError(evt.Error, m.err) {
		return false
	}

	want := snapshotUpdated(m.vm, m.phase, evt.Error)

	return *evt == *want
}

func (m snapshotUpdatedMatcher) String() string {
	return "is the snapshot updated event for phase " + string(m.phase) + " with an error that holds " + m.err
}

func hasError(got, want string) bool {
	if want == "" {
		return got == ""
	}

	return strings.Contains(got, want)
}

func pendingRecord(vm *models.MicroVM) *models.Snapshot {
	record := captureRecord(vm)
	record.Status.Phase = models.SnapshotPhasePending

	return record
}

func snapshotUpdated(vm *models.MicroVM, phase models.SnapshotPhase, errMsg string) *events.SnapshotUpdated {
	return &events.SnapshotUpdated{
		UID:       testSnapshotUID,
		Name:      "snap1",
		Namespace: vm.ID.Namespace(),
		SourceUID: vm.ID.UID(),
		Phase:     string(phase),
		Error:     errMsg,
	}
}

func expectPhase(m captureAppMocks, vm *models.MicroVM, phase models.SnapshotPhase, errMsg string) []*gomock.Call {
	return []*gomock.Call{
		m.snapshots.EXPECT().Update(gomock.Any(), snapshotPhaseMatcher{phase: phase, err: errMsg}).Return(nil),
		m.events.EXPECT().
			Publish(gomock.Any(), defaults.TopicSnapshotEvents, snapshotUpdatedMatcher{vm: vm, phase: phase, err: errMsg}).
			Return(nil),
	}
}

func TestApp_ReconcileSnapshot_PendingToReady(t *testing.T) {
	RegisterTestingT(t)

	app, m := newCaptureTestApp(t, &application.Config{})
	vm := snapshotSourceVM()

	m.snapshots.EXPECT().Get(gomock.Any(), models.SnapshotRef{UID: testSnapshotUID}).Return(pendingRecord(vm), nil)
	m.repo.EXPECT().Get(gomock.Any(), ports.RepositoryGetOptions{UID: testUID}).Return(vm, nil).Times(2)
	m.provider.EXPECT().Drives(gomock.Any()).Return(models.Drives{{ID: "rw", Role: models.DriveRoleRootRW}}, nil)

	calls := expectPhase(m, vm, models.SnapshotPhaseCapturing, "")
	calls = append(calls,
		m.provider.EXPECT().State(gomock.Any(), vm.ID.String()).Return(ports.MicroVMStateRunning, nil),
		m.provider.EXPECT().Pause(gomock.Any(), vm).Return(nil),
		m.provider.EXPECT().Snapshot(gomock.Any(), vm, captureTestDir).Return(nil),
		m.volumes.EXPECT().Capture(gomock.Any(), ports.VolumeRef{VMID: vm.ID, VolumeID: "root"}, captureTestDir).
			Return(&models.CapturedVolume{BaseDigest: "abc"}, nil),
		m.provider.EXPECT().Resume(gomock.Any(), vm).Return(nil),
		m.hostInfo.EXPECT().Facts(gomock.Any(), "mock").Return(&models.HostFacts{}, nil),
		m.images.EXPECT().Digest(gomock.Any(), gomock.Any()).Return("sha256:abc", nil),
	)
	calls = append(calls, expectPhase(m, vm, models.SnapshotPhaseReady, "")...)
	gomock.InOrder(calls...)

	Expect(app.ReconcileSnapshot(context.Background(), testSnapshotUID)).To(Succeed())
}

func TestApp_ReconcileSnapshot_CaptureFails(t *testing.T) {
	RegisterTestingT(t)

	app, m := newCaptureTestApp(t, &application.Config{})
	vm := snapshotSourceVM()

	m.snapshots.EXPECT().Get(gomock.Any(), gomock.Any()).Return(pendingRecord(vm), nil)
	m.repo.EXPECT().Get(gomock.Any(), gomock.Any()).Return(vm, nil).Times(2)

	calls := expectPhase(m, vm, models.SnapshotPhaseCapturing, "")
	calls = append(calls,
		m.provider.EXPECT().State(gomock.Any(), vm.ID.String()).Return(ports.MicroVMStateRunning, nil),
		m.provider.EXPECT().Pause(gomock.Any(), vm).Return(nil),
		m.provider.EXPECT().Snapshot(gomock.Any(), vm, captureTestDir).Return(errCaptureTest),
		m.provider.EXPECT().Resume(gomock.Any(), vm).Return(nil),
		m.snapshots.EXPECT().RemoveFiles(gomock.Any(), testSnapshotUID).Return(nil),
	)
	calls = append(calls, expectPhase(m, vm, models.SnapshotPhaseFailed, errCaptureTest.Error())...)
	gomock.InOrder(calls...)

	Expect(app.ReconcileSnapshot(context.Background(), testSnapshotUID)).To(Succeed())
}

func TestApp_ReconcileSnapshot_SourceGone(t *testing.T) {
	RegisterTestingT(t)

	app, m := newCaptureTestApp(t, &application.Config{})
	vm := snapshotSourceVM()

	m.snapshots.EXPECT().Get(gomock.Any(), gomock.Any()).Return(pendingRecord(vm), nil)
	m.repo.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, nil)

	calls := expectPhase(m, vm, models.SnapshotPhaseCapturing, "")
	calls = append(calls, expectPhase(m, vm, models.SnapshotPhaseFailed, "not found")...)
	gomock.InOrder(calls...)

	Expect(app.ReconcileSnapshot(context.Background(), testSnapshotUID)).To(Succeed())
}

func TestApp_ReconcileSnapshot_NothingToDo(t *testing.T) {
	for _, phase := range []models.SnapshotPhase{
		models.SnapshotPhaseCapturing,
		models.SnapshotPhaseReady,
		models.SnapshotPhaseFailed,
	} {
		t.Run(string(phase), func(t *testing.T) {
			RegisterTestingT(t)

			app, m := newCaptureTestApp(t, &application.Config{})
			record := pendingRecord(snapshotSourceVM())
			record.Status.Phase = phase

			m.snapshots.EXPECT().Get(gomock.Any(), gomock.Any()).Return(record, nil)

			Expect(app.ReconcileSnapshot(context.Background(), testSnapshotUID)).To(Succeed())
		})
	}
}

func TestApp_ReconcileSnapshot_Deleted(t *testing.T) {
	RegisterTestingT(t)

	app, m := newCaptureTestApp(t, &application.Config{})

	m.snapshots.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, coreerrs.NewSnapshotNotFound(testSnapshotUID))

	Expect(app.ReconcileSnapshot(context.Background(), testSnapshotUID)).To(Succeed())
}

func TestApp_ReconcileSnapshot_GetFails(t *testing.T) {
	RegisterTestingT(t)

	app, m := newCaptureTestApp(t, &application.Config{})

	m.snapshots.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, errCaptureTest)

	Expect(app.ReconcileSnapshot(context.Background(), testSnapshotUID)).To(MatchError(errCaptureTest))
}

func TestApp_ReconcileSnapshot_CapturingUpdateFails(t *testing.T) {
	RegisterTestingT(t)

	app, m := newCaptureTestApp(t, &application.Config{})
	vm := snapshotSourceVM()
	updateErr := errors.New("disk full")

	m.snapshots.EXPECT().Get(gomock.Any(), gomock.Any()).Return(pendingRecord(vm), nil)
	m.snapshots.EXPECT().Update(gomock.Any(), gomock.Any()).Return(updateErr)

	Expect(app.ReconcileSnapshot(context.Background(), testSnapshotUID)).To(MatchError(updateErr))
}

func TestApp_ReconcileSnapshot_PublishFailureDoesNotStopTheCapture(t *testing.T) {
	RegisterTestingT(t)

	app, m := newCaptureTestApp(t, &application.Config{})
	vm := snapshotSourceVM()

	m.snapshots.EXPECT().Get(gomock.Any(), gomock.Any()).Return(pendingRecord(vm), nil)
	m.repo.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, nil)
	m.snapshots.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil).Times(2)
	m.events.EXPECT().Publish(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("bus down")).Times(2)

	Expect(app.ReconcileSnapshot(context.Background(), testSnapshotUID)).To(Succeed())
}
