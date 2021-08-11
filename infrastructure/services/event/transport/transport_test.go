package transport_test

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/weaveworks/reignite/core/models"
	"github.com/weaveworks/reignite/core/ports"
	"github.com/weaveworks/reignite/infrastructure/services/event/transport"
)

func TestTransport_SimplePubSub(t *testing.T) {
	RegisterTestingT(t)

	ctx := context.Background()
	trans := transport.New()

	topicName := "test"
	messageReceived := false
	errorReceived := false

	handler := func(e *models.EventEnvelope) {
		messageReceived = true
	}
	errHandler := func(err error) {
		errorReceived = true
	}

	err := trans.CreateTopic(ctx, topicName)
	Expect(err).NotTo(HaveOccurred())

	err = trans.Subscribe(ctx, topicName, ports.EventHandlers{
		Event: handler,
		Error: errHandler,
	})
	Expect(err).NotTo(HaveOccurred())

	err = trans.Publish(ctx, topicName, "someevent")
	Expect(err).NotTo(HaveOccurred())

	time.Sleep(1 * time.Second)

	Expect(messageReceived).To(BeTrue())
	Expect(errorReceived).To(BeFalse())
}

func TestTransport_MultipleSubscribers(t *testing.T) {
	RegisterTestingT(t)

	ctx := context.Background()
	trans := transport.New()

	topicName := "test"
	sub1MessageReceived := false
	sub2MessageReceived := false
	errorReceived := false

	handler1 := func(e *models.EventEnvelope) {
		sub1MessageReceived = true
	}
	handler2 := func(e *models.EventEnvelope) {
		sub2MessageReceived = true
	}
	errHandler := func(err error) {
		errorReceived = true
	}

	err := trans.CreateTopic(ctx, topicName)
	Expect(err).NotTo(HaveOccurred())

	err = trans.Subscribe(ctx, topicName, ports.EventHandlers{
		Event: handler1,
		Error: errHandler,
	})
	Expect(err).NotTo(HaveOccurred())

	err = trans.Subscribe(ctx, topicName, ports.EventHandlers{
		Event: handler2,
		Error: errHandler,
	})
	Expect(err).NotTo(HaveOccurred())

	err = trans.Publish(ctx, topicName, "someevent")
	Expect(err).NotTo(HaveOccurred())

	time.Sleep(1 * time.Second)

	Expect(sub1MessageReceived).To(BeTrue())
	Expect(sub2MessageReceived).To(BeTrue())
	Expect(errorReceived).To(BeFalse())
}

func TestTransport_SubscribeUnknownTopic(t *testing.T) {
	RegisterTestingT(t)

	ctx := context.Background()
	trans := transport.New()

	topicName := "doesntexist"

	handler := func(e *models.EventEnvelope) {}

	errHandler := func(err error) {}

	err := trans.Subscribe(ctx, topicName, ports.EventHandlers{
		Event: handler,
		Error: errHandler,
	})
	Expect(err).To(HaveOccurred())
}

func TestTransport_SubscribeEmptyTopic(t *testing.T) {
	RegisterTestingT(t)

	ctx := context.Background()
	trans := transport.New()

	topicName := ""

	handler := func(e *models.EventEnvelope) {}

	errHandler := func(err error) {}

	err := trans.Subscribe(ctx, topicName, ports.EventHandlers{
		Event: handler,
		Error: errHandler,
	})
	Expect(err).To(HaveOccurred())
}

func TestTransport_PublishUnknownTopic(t *testing.T) {
	RegisterTestingT(t)

	ctx := context.Background()
	trans := transport.New()

	topicName := "doesntexist"

	err := trans.Publish(ctx, topicName, "someevent")
	Expect(err).To(HaveOccurred())
}

func TestTransport_PublishEmptyTopic(t *testing.T) {
	RegisterTestingT(t)

	ctx := context.Background()
	trans := transport.New()

	topicName := ""

	err := trans.Publish(ctx, topicName, "someevent")
	Expect(err).To(HaveOccurred())
}

func TestTransport_IdempotentCreateTopic(t *testing.T) {
	RegisterTestingT(t)

	ctx := context.Background()
	trans := transport.New()

	topicName := "test"

	err := trans.CreateTopic(ctx, topicName)
	Expect(err).NotTo(HaveOccurred(), "creating topic first time should succeed")

	err = trans.CreateTopic(ctx, topicName)
	Expect(err).NotTo(HaveOccurred(), "creating topic again time should succeed")
}

func TestTransport_CreateEmptyTopic(t *testing.T) {
	RegisterTestingT(t)

	ctx := context.Background()
	trans := transport.New()

	topicName := ""

	err := trans.CreateTopic(ctx, topicName)
	Expect(err).To(HaveOccurred(), "creating topic with a blank name should fail")
}

func TestTransport_SubscribeNilHandler(t *testing.T) {
	RegisterTestingT(t)

	ctx := context.Background()
	trans := transport.New()

	topicName := "test"

	if err := trans.CreateTopic(ctx, topicName); err != nil {
		t.Fatal(err)
	}

	errHandler := func(err error) {}

	err := trans.Subscribe(ctx, topicName, ports.EventHandlers{
		Error: errHandler,
	})
	Expect(err).To(HaveOccurred())
}

func TestTransport_SubscribeNilErrorHandler(t *testing.T) {
	RegisterTestingT(t)

	ctx := context.Background()
	trans := transport.New()

	topicName := "test"

	if err := trans.CreateTopic(ctx, topicName); err != nil {
		t.Fatal(err)
	}

	handler := func(e *models.EventEnvelope) {}

	err := trans.Subscribe(ctx, topicName, ports.EventHandlers{
		Event: handler,
	})
	Expect(err).To(HaveOccurred())
}
