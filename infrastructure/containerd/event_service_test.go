package containerd_test

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/weaveworks/reignite/api/events"
	"github.com/weaveworks/reignite/core/ports"
	"github.com/weaveworks/reignite/infrastructure/containerd"
)

func TestEventService_Integration(t *testing.T) {
	if !runContainerDTests() {
		t.Skip("skipping containerd event service integration test")
	}

	RegisterTestingT(t)

	client, ctx := testCreateClient(t)

	es := containerd.NewEventServiceWithClient(client)

	t.Log("creating subscribers")

	ctx1, cancel1 := context.WithCancel(ctx)
	evt1, err1 := es.Subscribe(ctx1)
	ctx2, cancel2 := context.WithCancel(ctx)
	evt2, err2 := es.Subscribe(ctx2)

	errChan := make(chan error)

	testEvents := []*events.MicroVMSpecCreated{
		{
			ID:        "vm1",
			Namespace: "ns1",
		},
		{
			ID:        "vm2",
			Namespace: "ns1",
		},
	}

	go func() {
		defer close(errChan)
		for _, event := range testEvents {
			if err := es.Publish(ctx, "/reignite/test", event); err != nil {
				errChan <- err
				return
			}
		}

		t.Log("finished publishing events")
	}()

	t.Log("subscribers waiting for events")
	if err := <-errChan; err != nil {
		t.Fatal(err)
	}

	for _, subscriber := range []struct {
		eventCh    <-chan *ports.EventEnvelope
		eventErrCh <-chan error
		cancel     func()
	}{
		{
			eventCh:    evt1,
			eventErrCh: err1,
			cancel:     cancel1,
		},
		{
			eventCh:    evt2,
			eventErrCh: err2,
			cancel:     cancel2,
		},
	} {
		recvd := []interface{}{}
	subscibercheck:
		for {
			select {
			case env := <-subscriber.eventCh:
				if env != nil {
					recvd = append(recvd, env.Event)
				} else {
					break subscibercheck
				}
			case err := <-subscriber.eventErrCh:
				if err != nil {
					t.Fatal(err)
				}
				break subscibercheck
			}

			if len(recvd) == len(testEvents) {
				subscriber.cancel()
			}
		}
	}
}

type testEvent struct {
	Name  string
	Value string
}
