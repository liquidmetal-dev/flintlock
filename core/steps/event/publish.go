package event

import (
	"context"
	"fmt"

	"github.com/liquidmetal-dev/flintlock/core/ports"
	"github.com/liquidmetal-dev/flintlock/pkg/log"
	"github.com/liquidmetal-dev/flintlock/pkg/planner"
	"github.com/sirupsen/logrus"
)

func NewPublish(topic string, event interface{}, eventSvc ports.EventService) planner.Procedure {
	return &eventPublish{
		event:    event,
		eventSvc: eventSvc,
		topic:    topic,
	}
}

type eventPublish struct {
	topic    string
	event    interface{}
	eventSvc ports.EventService
}

// Name is the name of the procedure/operation.
func (s *eventPublish) Name() string {
	return "event_publish"
}

func (s *eventPublish) ShouldDo(ctx context.Context) (bool, error) {
	return true, nil
}

// Do will perform the operation/procedure.
func (s *eventPublish) Do(ctx context.Context) ([]planner.Procedure, error) {
	logger := log.GetLogger(ctx).WithFields(logrus.Fields{
		"step": s.Name(),
	})
	logger.Debug("running step to publish event")

	if err := s.eventSvc.Publish(ctx, s.topic, s.event); err != nil {
		return nil, fmt.Errorf("publishing event to topic %s: %w", s.topic, err)
	}

	return nil, nil
}

func (s *eventPublish) Verify(ctx context.Context) error {
	return nil
}
