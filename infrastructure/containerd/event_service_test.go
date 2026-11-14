package containerd_test

import (
	"context"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/liquidmetal-dev/flintlock/api/events"
	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/infrastructure/containerd"
)

const (
	numberOfSubscribers        = 2
	sleepTime                  = 40
	subscriberWait             = 30
	subscriberTimeoutInSeconds = 20
)

func TestEventService_Integration(t *testing.T) {
	if !runContainerDTests() {
		t.Skip("skipping containerd event service integration test")
	}

	RegisterTestingT(t)

	client, ctx := testCreateClient(t)

	es := containerd.NewEventServiceWithClient(&containerd.Config{
		SnapshotterKernel: testSnapshotter,
		SnapshotterVolume: testSnapshotter,
		Namespace:         testContainerdNS,
	}, client)

	testEvents := []*events.MicroVMSpecCreated{
		{ID: "vm1", Namespace: "ns1"},
		{ID: "vm2", Namespace: "ns1"},
		{ID: "vm1", Namespace: "ns2"},
	}

	var (
		wgReady sync.WaitGroup
		wgDone  sync.WaitGroup
	)

	t.Log("creating subscribers")

	for i := 0; i < numberOfSubscribers; i++ {
		wgReady.Add(1)
		wgDone.Add(1)

		data := subData{
			ID:        i,
			ES:        es,
			MaxEvents: len(testEvents),
			Ready:     wgReady.Done,
			Done:      wgDone.Done,
		}

		go newSubscriber(t, ctx, data)
	}

	wgReady.Wait()

	// Without this, it's still possible we publish the first ever before the
	// connection is read.
	time.Sleep(time.Millisecond * sleepTime)

	t.Log("publishing events")

	for _, event := range testEvents {
		t.Logf("publishing event: %v", event)
		if err := es.Publish(ctx, "/flintlock/test", event); err != nil {
			t.Fatal(err)
			break
		}
	}

	t.Log("finished publishing events")

	wgDone.Wait()
}

type subData struct {
	ID        int
	ES        ports.EventService
	MaxEvents int
	Ready     func()
	Done      func()
}

func newSubscriber(t *testing.T, rootContext context.Context, data subData) {
	ctx, cancel := context.WithCancel(rootContext)
	evtChan, errChan := data.ES.Subscribe(ctx)

	subscriber := testSubscriber{
		eventCh:    evtChan,
		eventErrCh: errChan,
		cancel:     cancel,
	}

	t.Logf("subscriber (%d) is ready to receive events", data.ID)

	data.Ready()
	defer data.Done()

	recvd, err := watch(&subscriber, data.MaxEvents)

	t.Logf("subscriber (%d) is done", data.ID)

	Expect(err).To(BeNil())
	Expect(recvd).To(HaveLen(data.MaxEvents))
}

func watch(subscriber *testSubscriber, maxEvents int) ([]interface{}, error) {
	recvd := []interface{}{}
	start := time.Now()

	var err error

mainloop:
	for {
		select {
		case env := <-subscriber.eventCh:
			if env == nil {
				break
			}
			recvd = append(recvd, env.Event)
		case err = <-subscriber.eventErrCh:
			break
		default:
			if time.Since(start).Seconds() > subscriberTimeoutInSeconds {
				subscriber.cancel()

				break mainloop
			}

			time.Sleep(time.Microsecond * subscriberWait)
		}

		if len(recvd) == maxEvents {
			subscriber.cancel()

			break
		}
	}

	return recvd, err
}

type testEvent struct {
	Name  string
	Value string
}

type testSubscriber struct {
	eventCh    <-chan *ports.EventEnvelope
	eventErrCh <-chan error
	cancel     func()
}
