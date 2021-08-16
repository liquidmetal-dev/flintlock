package transport

import (
	"context"
	"fmt"

	"github.com/vmware/transport-go/bus"
	"github.com/vmware/transport-go/model"

	event "github.com/weaveworks/reignite/core"
	"github.com/weaveworks/reignite/core/models"
	"github.com/weaveworks/reignite/core/ports"
)

// New creates a new event service based on Transport (https://vmware.github.io/transport/).
func New() ports.EventService {
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
func (te *transportEvents) Publish(ctx context.Context, topic string, eventToPublish interface{}) error {
	if topic == "" {
		return errTopicRequired
	}

	manager := te.eventBus.GetChannelManager()

	if !manager.CheckChannelExists(topic) {
		return event.ErrTopicNotFound{Name: topic}
	}

	if err := te.eventBus.SendRequestMessage(topic, eventToPublish, nil); err != nil {
		return fmt.Errorf("sending message to channel: %w", err)
	}

	return nil
}

// Subscribe will subscribe to events on a named topic and will call the relevant handler.
func (te *transportEvents) Subscribe(ctx context.Context, topic string, handlers ports.EventHandlers) error {
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
		return event.ErrTopicNotFound{Name: topic}
	}

	h, err := te.eventBus.ListenRequestStream(topic)
	if err != nil {
		return fmt.Errorf("listening for transport events: %w", err)
	}
	h.Handle(te.subsciberHandler(handlers.Event), te.subsciberErrorHandler(handlers.Error))

	return nil
}

func (te *transportEvents) subsciberHandler(handler ports.EventHandler) bus.MessageHandlerFunction {
	return func(msg *model.Message) {
		evt := &models.EventEnvelope{
			ID:    msg.Id.String(),
			Topic: msg.Channel,
			Event: msg.Payload,
		}

		handler(evt)
	}
}

func (te *transportEvents) subsciberErrorHandler(errHandler ports.EventErrorHandler) bus.MessageErrorFunction {
	return func(err error) {
		errHandler(err)
	}
}
