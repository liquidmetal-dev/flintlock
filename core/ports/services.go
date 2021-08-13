package ports

import (
	"context"

	mvmv1 "github.com/weaveworks/reignite/api/services/microvm/v1alpha1"
	"github.com/weaveworks/reignite/core/models"
)

// MicroVMGRPCService is a port for a microvm grpc service.
type MicroVMGRPCService interface {
	mvmv1.MicroVMServer
}

// IDService is a port for a service for working with identifiers.
type IDService interface {
	// GenerateRandom will create a random identifier.
	GenerateRandom() (string, error)
}

// EventHandler represents an event handling function.
type EventHandler func(e *models.EventEnvelope)

// EventErrorHandler represents an error handling function.
type EventErrorHandler func(err error)

// EventHandlers represents a pair of event/error handlers.
type EventHandlers struct {
	// Event is the event handler function.
	Event EventHandler
	// Error is the error handler function.
	Error EventErrorHandler
}

// EventService is a port for a service that acts as a event bus.
type EventService interface {
	// CreateTopic will create a named topic (a.k.a channel or queue) for events.
	CreateTopic(ctx context.Context, topic string) error
	// Publish will publish an event to a specific topic.
	Publish(ctx context.Context, topic string, eventToPublish interface{}) error
	// Subscribe will subscribe to events on a named topic and will call the relevant handlers.
	Subscribe(ctx context.Context, topic string, handlers EventHandlers) error
}
