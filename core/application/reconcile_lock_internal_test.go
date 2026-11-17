package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	g "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/api/events"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/infrastructure/mock"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
)

const lockTestUID = "0d4b8f0e-6a4c-4e4b-9a8e-2f6d5c1b7a90"

var errLockTest = errors.New("store unavailable")

func newLockTestApp(t *testing.T) (*app, *mock.MockMicroVMRepository, *mock.MockEventService) {
	t.Helper()

	ctrl := gomock.NewController(t)
	repo := mock.NewMockMicroVMRepository(ctrl)
	eventSvc := mock.NewMockEventService(ctrl)

	a, _ := New(&Config{}, &ports.Collection{Repo: repo, EventService: eventSvc}).(*app)
	a.lockedRequeueDelay = 10 * time.Millisecond

	return a, repo, eventSvc
}

func TestReconcileMicroVM_RequeuesWhileLocked(t *testing.T) {
	g.RegisterTestingT(t)

	a, _, eventSvc := newLockTestApp(t)
	vmid, _ := models.NewVMID("vm1", "ns1", lockTestUID)

	published := make(chan struct{})
	eventSvc.EXPECT().
		Publish(gomock.Any(), defaults.TopicMicroVMEvents, &events.MicroVMSpecUpdated{
			ID: "vm1", Namespace: "ns1", UID: lockTestUID,
		}).
		DoAndReturn(func(context.Context, string, interface{}) error {
			close(published)

			return nil
		})

	a.vmLocks.Lock(lockTestUID)
	defer a.vmLocks.Unlock(lockTestUID)

	start := time.Now()

	g.Expect(a.ReconcileMicroVM(context.Background(), *vmid)).To(g.Succeed())
	g.Eventually(published).Should(g.BeClosed())
	g.Expect(time.Since(start)).To(g.BeNumerically(">=", a.lockedRequeueDelay))
}

func TestReconcileMicroVM_HoldsTheLock(t *testing.T) {
	g.RegisterTestingT(t)

	a, repo, _ := newLockTestApp(t)
	vmid, _ := models.NewVMID("vm1", "ns1", lockTestUID)

	repo.EXPECT().Get(gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, ports.RepositoryGetOptions) (*models.MicroVM, error) {
			g.Expect(a.vmLocks.TryLock(lockTestUID)).To(g.BeFalse(), "the reconcile holds the lock")

			return nil, errLockTest
		})

	g.Expect(a.ReconcileMicroVM(context.Background(), *vmid)).To(g.MatchError(errLockTest))

	g.Expect(a.vmLocks.TryLock(lockTestUID)).To(g.BeTrue(), "the reconcile released the lock")
	a.vmLocks.Unlock(lockTestUID)
}

func TestDefaultLockedRequeueDelay(t *testing.T) {
	g.RegisterTestingT(t)

	a, _ := New(&Config{}, &ports.Collection{}).(*app)

	g.Expect(a.lockedRequeueDelay).To(g.Equal(5 * time.Second))
}

func TestRecoverSnapshots_ReadsTheSourceUnderTheLock(t *testing.T) {
	g.RegisterTestingT(t)

	ctrl := gomock.NewController(t)
	repo := mock.NewMockMicroVMRepository(ctrl)
	snapshots := mock.NewMockSnapshotRepository(ctrl)
	eventSvc := mock.NewMockEventService(ctrl)
	a, _ := New(&Config{}, &ports.Collection{
		Repo: repo, SnapshotRepo: snapshots, EventService: eventSvc,
	}).(*app)

	vmid, _ := models.NewVMID("vm1", "ns1", lockTestUID)
	snapshots.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*models.Snapshot{{
		UID: "snap1", Source: *vmid, Status: models.SnapshotStatus{Phase: models.SnapshotPhaseCapturing},
	}}, nil)
	repo.EXPECT().Get(gomock.Any(), ports.RepositoryGetOptions{UID: lockTestUID}).
		DoAndReturn(func(context.Context, ports.RepositoryGetOptions) (*models.MicroVM, error) {
			g.Expect(a.vmLocks.TryLock(lockTestUID)).To(g.BeFalse(), "recovery holds the lock")

			return nil, nil
		})
	snapshots.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	eventSvc.EXPECT().Publish(gomock.Any(), defaults.TopicSnapshotEvents, gomock.Any()).Return(nil)
	snapshots.EXPECT().RemoveFiles(gomock.Any(), "snap1").Return(nil)

	g.Expect(a.RecoverSnapshots(context.Background())).To(g.Succeed())

	g.Expect(a.vmLocks.TryLock(lockTestUID)).To(g.BeTrue(), "recovery released the lock")
	a.vmLocks.Unlock(lockTestUID)
}
