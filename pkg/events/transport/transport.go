package transport

import (
	"context"
	"fmt"

	"github.com/vmware/transport-go/bus"
	"github.com/vmware/transport-go/model"

	"github.com/weaveworks/reignite/pkg/events"
)

// New creates a new event bus based on Transport (https://vmware.github.io/transport/).
func New() events.EventBus {
	return &transportEvents{
		eventBus: bus.GetBus(),
	}
}

type transportEvents struct {
	eventBus bus.EventBus
}

// CreateTopic will create a named topic (a.k.a channel or queue) for events.
func (te *transportEvents) CreateTopic(ctx context.Context, topic string) error {
	if topic == "" {
		return errTopicRequired
	}

	manager := te.eventBus.GetChannelManager()

	if !manager.CheckChannelExists(topic) {
		manager.CreateChannel(topic)
	}

	return nil
}

// Publish will publish an event to a specific topic.
func (te *transportEvents) Publish(ctx context.Context, topic string, event interface{}) error {
	if topic == "" {
		return errTopicRequired
	}

	manager := te.eventBus.GetChannelManager()

	if !manager.CheckChannelExists(topic) {
		return events.ErrTopicNotFound{Name: topic}
	}

	if err := te.eventBus.SendRequestMessage(topic, event, nil); err != nil {
		return fmt.Errorf("sending message to channel: %w", err)
	}

	return nil
}

// Subscribe will subscribe to events on a named topic and will call the relevant handler.
func (te *transportEvents) Subscribe(ctx context.Context, topic string, handlers events.Handlers) error {
	if handlers.Event == nil {
		return errHandlerRequired
	}
	if handlers.Error == nil {
		return errErrorHandlerRequired
	}
	if topic == "" {
		return errTopicRequired
	}

	manager := te.eventBus.GetChannelManager()

	if !manager.CheckChannelExists(topic) {
		return events.ErrTopicNotFound{Name: topic}
	}

	h, err := te.eventBus.ListenRequestStream(topic)
	if err != nil {
		return fmt.Errorf("listening for transport events: %w", err)
	}
	h.Handle(te.subsciberHandler(handlers.Event), te.subsciberErrorHandler(handlers.Error))

	return nil
}

func (te *transportEvents) subsciberHandler(handler events.Handler) bus.MessageHandlerFunction {
	return func(msg *model.Message) {
		evt := &events.Envelope{
			ID:    *msg.Id,
			Topic: msg.Channel,
			Event: msg.Payload,
		}

		handler(evt)
	}
}

func (te *transportEvents) subsciberErrorHandler(errHandler events.ErrorHandler) bus.MessageErrorFunction {
	return func(err error) {
		errHandler(err)
	}
}
