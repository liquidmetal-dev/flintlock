package controllers_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	lgrtest "github.com/sirupsen/logrus/hooks/test"

	"github.com/liquidmetal-dev/flintlock/api/events"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/infrastructure/controllers"
	"github.com/liquidmetal-dev/flintlock/infrastructure/mock"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
)

const (
	snapUID1 = "snap-1"
	snapUID2 = "snap-2"
)

type snapshotControllerTest struct {
	ctx       context.Context
	hook      *lgrtest.Hook
	events    *mock.MockEventService
	reconcile *mock.MockReconcileSnapshotsUseCase
	query     *mock.MockSnapshotQueryUseCases
	evtCh     chan *ports.EventEnvelope
	evtErrCh  chan error
}

func newSnapshotControllerTest(t *testing.T) *snapshotControllerTest {
	t.Helper()

	ctrl := gomock.NewController(t)
	logger, hook := lgrtest.NewNullLogger()

	return &snapshotControllerTest{
		ctx:       log.WithLogger(context.Background(), logger.WithField("test", t.Name())),
		hook:      hook,
		events:    mock.NewMockEventService(ctrl),
		reconcile: mock.NewMockReconcileSnapshotsUseCase(ctrl),
		query:     mock.NewMockSnapshotQueryUseCases(ctrl),
		evtCh:     make(chan *ports.EventEnvelope),
		evtErrCh:  make(chan error, 1),
	}
}

func (s *snapshotControllerTest) expectSubscribe() {
	s.events.EXPECT().SubscribeTopic(gomock.Any(), defaults.TopicSnapshotEvents).Return(s.evtCh, s.evtErrCh)
}

// run starts the controller and returns a function that stops it and returns
// the error of Run.
func (s *snapshotControllerTest) run(workers int, resyncPeriod time.Duration) func() error {
	ctx, cancel := context.WithCancel(s.ctx)
	controller := controllers.NewSnapshotController(s.events, s.reconcile, s.query)

	var (
		wg     sync.WaitGroup
		runErr error
	)

	wg.Add(1)

	go func() {
		defer wg.Done()

		runErr = controller.Run(ctx, workers, resyncPeriod)
	}()

	return func() error {
		cancel()
		wg.Wait()

		return runErr
	}
}

func snapshotCreatedEvent(uid string) *ports.EventEnvelope {
	return &ports.EventEnvelope{
		Timestamp: time.Now(),
		Namespace: ctrNS,
		Topic:     defaults.TopicSnapshotEvents,
		Event:     &events.SnapshotCreated{UID: uid, Name: uid, Namespace: vmNS, SourceUID: vmUID},
	}
}

func snapshotInPhase(uid string, phase models.SnapshotPhase) *models.Snapshot {
	return &models.Snapshot{UID: uid, Status: models.SnapshotStatus{Phase: phase}}
}

func TestSnapshotController_CreatedEventCausesReconcile(t *testing.T) {
	RegisterTestingT(t)

	s := newSnapshotControllerTest(t)
	s.expectSubscribe()
	s.reconcile.EXPECT().RecoverSnapshots(gomock.Any()).Return(nil)
	s.query.EXPECT().GetAllSnapshots(gomock.Any(), gomock.Any()).Return(nil, nil)

	reconciled := make(chan struct{})
	s.reconcile.EXPECT().ReconcileSnapshot(gomock.Any(), snapUID1).DoAndReturn(func(context.Context, string) error {
		close(reconciled)

		return nil
	})

	stop := s.run(1, time.Hour)
	s.evtCh <- snapshotCreatedEvent(snapUID1)

	Eventually(reconciled).Should(BeClosed())
	Expect(stop()).To(Succeed())
	Expect(hasLogError(s.hook)).To(BeFalse())
}

func TestSnapshotController_OtherEventsAreIgnored(t *testing.T) {
	RegisterTestingT(t)

	s := newSnapshotControllerTest(t)
	s.expectSubscribe()
	s.reconcile.EXPECT().RecoverSnapshots(gomock.Any()).Return(nil)
	s.query.EXPECT().GetAllSnapshots(gomock.Any(), gomock.Any()).Return(nil, nil)

	stop := s.run(1, time.Hour)
	s.evtCh <- &ports.EventEnvelope{Topic: defaults.TopicSnapshotEvents, Event: &events.SnapshotUpdated{UID: snapUID1}}
	s.evtCh <- &ports.EventEnvelope{Topic: defaults.TopicSnapshotEvents, Event: &events.SnapshotDeleted{UID: snapUID1}}
	time.Sleep(20 * time.Millisecond)

	Expect(stop()).To(Succeed())
}

func TestSnapshotController_RecoversBeforeAnyCapture(t *testing.T) {
	RegisterTestingT(t)

	s := newSnapshotControllerTest(t)
	s.expectSubscribe()

	reconciled := make(chan struct{})

	gomock.InOrder(
		s.reconcile.EXPECT().RecoverSnapshots(gomock.Any()).Return(errors.New("one snapshot could not be recovered")),
		s.query.EXPECT().GetAllSnapshots(gomock.Any(), ports.SnapshotListOptions{}).
			Return([]*models.Snapshot{snapshotInPhase(snapUID1, models.SnapshotPhasePending)}, nil),
		s.reconcile.EXPECT().ReconcileSnapshot(gomock.Any(), snapUID1).DoAndReturn(func(context.Context, string) error {
			close(reconciled)

			return nil
		}),
	)

	stop := s.run(1, time.Hour)

	Eventually(reconciled).Should(BeClosed())
	Expect(stop()).To(Succeed())
	Expect(hasLogError(s.hook)).To(BeTrue(), "the recovery error is logged")
}

func TestSnapshotController_ResyncQueuesPendingSnapshots(t *testing.T) {
	RegisterTestingT(t)

	s := newSnapshotControllerTest(t)
	s.expectSubscribe()
	s.reconcile.EXPECT().RecoverSnapshots(gomock.Any()).Return(nil)

	reconciled := make(chan struct{})
	var once sync.Once

	gomock.InOrder(
		s.query.EXPECT().GetAllSnapshots(gomock.Any(), gomock.Any()).Return(nil, nil),
		s.query.EXPECT().GetAllSnapshots(gomock.Any(), gomock.Any()).Return([]*models.Snapshot{
			snapshotInPhase(snapUID1, models.SnapshotPhasePending),
			snapshotInPhase(snapUID2, models.SnapshotPhaseCapturing),
			snapshotInPhase("snap-ready", models.SnapshotPhaseReady),
			snapshotInPhase("snap-failed", models.SnapshotPhaseFailed),
		}, nil).MinTimes(1),
	)
	s.reconcile.EXPECT().ReconcileSnapshot(gomock.Any(), snapUID1).DoAndReturn(func(context.Context, string) error {
		once.Do(func() { close(reconciled) })

		return nil
	}).MinTimes(1)

	stop := s.run(1, 10*time.Millisecond)

	Eventually(reconciled).Should(BeClosed())
	Expect(stop()).To(Succeed())
}

func TestSnapshotController_LimitsConcurrentCaptures(t *testing.T) {
	for _, tc := range []struct {
		name         string
		workers      int
		wantInFlight int32
	}{
		{name: "limit 1, the second snapshot waits for the first", workers: 1, wantInFlight: 1},
		{name: "limit 2, both snapshots run at once", workers: 2, wantInFlight: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			s := newSnapshotControllerTest(t)
			s.expectSubscribe()
			s.reconcile.EXPECT().RecoverSnapshots(gomock.Any()).Return(nil)
			s.query.EXPECT().GetAllSnapshots(gomock.Any(), gomock.Any()).Return(nil, nil)

			var inFlight, maxInFlight, done atomic.Int32

			release := make(chan struct{})
			capture := func(context.Context, string) error {
				n := inFlight.Add(1)
				for {
					m := maxInFlight.Load()
					if n <= m || maxInFlight.CompareAndSwap(m, n) {
						break
					}
				}

				<-release
				inFlight.Add(-1)
				done.Add(1)

				return nil
			}

			s.reconcile.EXPECT().ReconcileSnapshot(gomock.Any(), snapUID1).DoAndReturn(capture)
			s.reconcile.EXPECT().ReconcileSnapshot(gomock.Any(), snapUID2).DoAndReturn(capture)

			stop := s.run(tc.workers, time.Hour)
			s.evtCh <- snapshotCreatedEvent(snapUID1)
			s.evtCh <- snapshotCreatedEvent(snapUID2)

			Eventually(inFlight.Load).Should(Equal(tc.wantInFlight))
			Consistently(inFlight.Load, 50*time.Millisecond).Should(Equal(tc.wantInFlight))

			close(release)

			Eventually(done.Load).Should(Equal(int32(2)))
			Expect(maxInFlight.Load()).To(Equal(tc.wantInFlight))
			Expect(stop()).To(Succeed())
		})
	}
}

func TestSnapshotController_ReconcileErrorIsLogged(t *testing.T) {
	RegisterTestingT(t)

	s := newSnapshotControllerTest(t)
	s.expectSubscribe()
	s.reconcile.EXPECT().RecoverSnapshots(gomock.Any()).Return(nil)
	s.query.EXPECT().GetAllSnapshots(gomock.Any(), gomock.Any()).Return(nil, nil)

	reconciled := make(chan struct{})
	s.reconcile.EXPECT().ReconcileSnapshot(gomock.Any(), snapUID1).DoAndReturn(func(context.Context, string) error {
		close(reconciled)

		return errors.New("store unavailable")
	})

	stop := s.run(1, time.Hour)
	s.evtCh <- snapshotCreatedEvent(snapUID1)

	Eventually(reconciled).Should(BeClosed())
	Expect(stop()).To(Succeed())
	Expect(hasLogError(s.hook)).To(BeTrue())
}

func TestSnapshotController_NeedsAWorker(t *testing.T) {
	RegisterTestingT(t)

	s := newSnapshotControllerTest(t)

	Expect(s.run(0, time.Hour)()).To(MatchError(ContainSubstring("at least 1")))
}
