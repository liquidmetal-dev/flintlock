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

// ImageService is a port for a service that interacts with OCI images.
type ImageService interface {
	// Get will get (i.e. pull) the image for a specific owner.
	Get(ctx context.Context, input GetImageInput) error
	// GetAndMount will get (i.e. pull) the image for a specific owner and then
	// make it available via a mount point.
	GetAndMount(ctx context.Context, input GetImageInput) ([]models.Mount, error)
}

// GetImageInput is the input to getting a image.
type GetImageInput struct {
	// ImageName is the name of the image to get.
	ImageName string
	// OwnerName is the name of the owner of the image.
	OwnerName string
	// OwnerNamespace is the namespace of the owner of the image.
	OwnerNamespace string
	// Use is an indoicator of what the image will be used for.
	Use models.ImageUse
}
