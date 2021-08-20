package ports

import (
	"context"
	"time"

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

// EventService is a port for a service that acts as a event bus.
type EventService interface {
	// Publish will publish an event to a specific topic.
	Publish(ctx context.Context, topic string, eventToPublish interface{}) error
	// SubscribeTopic will subscribe to events on a named topic..
	SubscribeTopic(ctx context.Context, topic string) (ch <-chan *EventEnvelope, errs <-chan error)
	// Subscribe will subscribe to events on all topics
	Subscribe(ctx context.Context) (ch <-chan *EventEnvelope, errs <-chan error)
}

type EventEnvelope struct {
	Timestamp time.Time
	Namespace string
	Topic     string
	Event     interface{}
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
