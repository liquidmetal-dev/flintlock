package controllers_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	lgrtest "github.com/sirupsen/logrus/hooks/test"

	"github.com/liquidmetal-dev/flintlock/api/events"
	"github.com/liquidmetal-dev/flintlock/core/models"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/infrastructure/controllers"
	"github.com/liquidmetal-dev/flintlock/infrastructure/mock"
	"github.com/liquidmetal-dev/flintlock/pkg/defaults"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
)

var (
	vmID  = "vm1"
	vmNS  = "testns"
	vmUID = "uid"
	ctrNS = "flintlock_test_controller"
)

func TestMicroVMController(t *testing.T) {
	testCases := []struct {
		name         string
		eventsToSend []*ports.EventEnvelope
		expectError  bool
		expectLogErr bool
		expect       func(em *mock.MockEventServiceMockRecorder, uc *mock.MockReconcileMicroVMsUseCaseMockRecorder, evtChan chan *ports.EventEnvelope, evtErrCh chan error)
	}{
		{
			name: "create event causes reconcile",
			eventsToSend: []*ports.EventEnvelope{
				createdEvent(vmID, vmNS),
			},
			expectError:  false,
			expectLogErr: false,
			expect: func(em *mock.MockEventServiceMockRecorder, uc *mock.MockReconcileMicroVMsUseCaseMockRecorder, evtChan chan *ports.EventEnvelope, evtErrCh chan error) {
				em.SubscribeTopic(gomock.Any(), gomock.Eq(defaults.TopicMicroVMEvents)).Return(evtChan, evtErrCh)

				uc.ReconcileMicroVM(gomock.Any(), gomock.Eq(*models.NewVMIDForce(vmID, vmNS, vmUID))).Return(nil)
			},
		},
		{
			name: "update event causes reconcile",
			eventsToSend: []*ports.EventEnvelope{
				updatedEvent(vmID, vmNS),
			},
			expectError: false,
			expect: func(em *mock.MockEventServiceMockRecorder, uc *mock.MockReconcileMicroVMsUseCaseMockRecorder, evtChan chan *ports.EventEnvelope, evtErrCh chan error) {
				em.SubscribeTopic(gomock.Any(), gomock.Eq(defaults.TopicMicroVMEvents)).Return(evtChan, evtErrCh)

				uc.ReconcileMicroVM(gomock.Any(), gomock.Eq(*models.NewVMIDForce(vmID, vmNS, vmUID))).Return(nil)
			},
		},
		{
			name: "delete event does not cause reconcile",
			eventsToSend: []*ports.EventEnvelope{
				deletedEvent(vmID, vmNS),
			},
			expectError: false,
			expect: func(em *mock.MockEventServiceMockRecorder, uc *mock.MockReconcileMicroVMsUseCaseMockRecorder, evtChan chan *ports.EventEnvelope, evtErrCh chan error) {
				em.SubscribeTopic(gomock.Any(), gomock.Eq(defaults.TopicMicroVMEvents)).Return(evtChan, evtErrCh)

				uc.ReconcileMicroVM(gomock.Any(), gomock.Eq(*models.NewVMIDForce(vmID, vmNS, vmUID))).Times(0)
			},
		},
		{
			name: "create event causes reconcile, 1st reconcile fails and then succeeds second time",
			eventsToSend: []*ports.EventEnvelope{
				createdEvent(vmID, vmNS),
			},
			expectError:  false,
			expectLogErr: true,
			expect: func(em *mock.MockEventServiceMockRecorder, uc *mock.MockReconcileMicroVMsUseCaseMockRecorder, evtChan chan *ports.EventEnvelope, evtErrCh chan error) {
				em.SubscribeTopic(gomock.Any(), gomock.Eq(defaults.TopicMicroVMEvents)).Return(evtChan, evtErrCh)

				failed := uc.ReconcileMicroVM(gomock.Any(), gomock.Eq(*models.NewVMIDForce(vmID, vmNS, vmUID))).Return(errors.New("something bad happened"))

				uc.ReconcileMicroVM(gomock.Any(), gomock.Eq(*models.NewVMIDForce(vmID, vmNS, vmUID))).Return(nil).After(failed)
			},
		},
		{
			name: "create event causes reconcile, 1st reconcile fails and then succeeds second time",
			eventsToSend: []*ports.EventEnvelope{
				createdEvent(vmID, vmNS),
			},
			expectError:  false,
			expectLogErr: true,
			expect: func(em *mock.MockEventServiceMockRecorder, uc *mock.MockReconcileMicroVMsUseCaseMockRecorder, evtChan chan *ports.EventEnvelope, evtErrCh chan error) {
				em.SubscribeTopic(gomock.Any(), gomock.Eq(defaults.TopicMicroVMEvents)).Return(evtChan, evtErrCh)

				failed := uc.ReconcileMicroVM(gomock.Any(), gomock.Eq(*models.NewVMIDForce(vmID, vmNS, vmUID))).Return(errors.New("something bad happened"))

				uc.ReconcileMicroVM(gomock.Any(), gomock.Eq(*models.NewVMIDForce(vmID, vmNS, vmUID))).Return(nil).After(failed)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			RegisterTestingT(t)

			ctx, cancel := context.WithCancel(context.Background())

			logger, hook := lgrtest.NewNullLogger()
			ctx = log.WithLogger(ctx, logger.WithField("test", tc.name))

			var err error
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			em := mock.NewMockEventService(mockCtrl)
			uc := mock.NewMockReconcileMicroVMsUseCase(mockCtrl)
			quc := mock.NewMockMicroVMQueryUseCases(mockCtrl)

			evtCh := make(chan *ports.EventEnvelope)
			evtErrCh := make(chan error, 1)

			tc.expect(em.EXPECT(), uc.EXPECT(), evtCh, evtErrCh)

			controller := controllers.New(em, uc, quc)

			ctrlWG := sync.WaitGroup{}
			ctrlWG.Add(1)
			go func() {
				defer ctrlWG.Done()
				err = controller.Run(ctx, 1, 10*time.Minute, false)
			}()

			for _, evt := range tc.eventsToSend {
				evtCh <- evt
				time.Sleep(3 * time.Millisecond)
			}

			cancel()
			ctrlWG.Wait()

			if tc.expectError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}

			if tc.expectLogErr {
				Expect(hasLogError(hook)).To(BeTrue())
			} else {
				Expect(hasLogError(hook)).To(BeFalse())
			}
		})
	}
}

func hasLogError(hook *lgrtest.Hook) bool {
	for _, entry := range hook.Entries {
		if entry.Level == logrus.ErrorLevel {
			return true
		}
	}

	return false
}

func createdEvent(name, namespace string) *ports.EventEnvelope {
	return &ports.EventEnvelope{
		Timestamp: time.Now(),
		Namespace: ctrNS,
		Topic:     defaults.TopicMicroVMEvents,
		Event: &events.MicroVMSpecCreated{
			ID:        name,
			Namespace: namespace,
			UID:       vmUID,
		},
	}
}

func updatedEvent(name, namespace string) *ports.EventEnvelope {
	return &ports.EventEnvelope{
		Timestamp: time.Now(),
		Namespace: ctrNS,
		Topic:     defaults.TopicMicroVMEvents,
		Event: &events.MicroVMSpecUpdated{
			ID:        name,
			Namespace: namespace,
			UID:       vmUID,
		},
	}
}

func deletedEvent(name, namespace string) *ports.EventEnvelope {
	return &ports.EventEnvelope{
		Timestamp: time.Now(),
		Namespace: ctrNS,
		Topic:     defaults.TopicMicroVMEvents,
		Event: &events.MicroVMSpecDeleted{
			ID:        name,
			Namespace: namespace,
			UID:       vmUID,
		},
	}
}
