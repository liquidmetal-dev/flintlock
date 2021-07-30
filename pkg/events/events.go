package events

import (
	"context"

	"github.com/google/uuid"
)

// Handler represents an event handling function.
type Handler func(e *Envelope)

// ErrorHandler represents an error handling function.
type ErrorHandler func(err error)

// Handlers represents a pair of event/error handlers.
type Handlers struct {
	// Event is the event handler function.
	Event Handler
	// Error is the error handler function.
	Error ErrorHandler
}

// EventBus is the interface that an event bus must implement.
type EventBus interface {
	// CreateTopic will create a named topic (a.k.a channel or queue) for events.
	CreateTopic(ctx context.Context, topic string) error
	// Publish will publish an event to a specific topic.
	Publish(ctx context.Context, topic string, event interface{}) error
	// Subscribe will subscribe to events on a named topic and will call the relevant handlers.
	Subscribe(ctx context.Context, topic string, handlers Handlers) error
}

// Envelope represents an event envelope.
type Envelope struct {
	// ID is the unique identifier for the event.
	ID uuid.UUID `json:"id"`
	// Topic is the name of the topic the event originated from.
	Topic string `json:"topic"`
	// Event is the actual event payload.
	Event interface{} `json:"event"`
}
