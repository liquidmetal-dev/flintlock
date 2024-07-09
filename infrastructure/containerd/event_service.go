package containerd

import (
	"context"
	"errors"
	"fmt"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/events"
	"github.com/containerd/containerd/namespaces"
	"github.com/liquidmetal-dev/flintlock/core/ports"
)

func NewEventService(cfg *Config) (ports.EventService, error) {
	client, err := containerd.New(cfg.SocketPath)
	if err != nil {
		return nil, fmt.Errorf("creating containerd client: %w", err)
	}

	return NewEventServiceWithClient(cfg, client), nil
}

func NewEventServiceWithClient(cfg *Config, client *containerd.Client) ports.EventService {
	return &eventService{
		client: client,
		cfg:    cfg,
	}
}

type eventService struct {
	client *containerd.Client
	cfg    *Config
}

// Publish will publish an event to a specific topic.
func (es *eventService) Publish(ctx context.Context, topic string, eventToPublish interface{}) error {
	namespaceCtx := namespaces.WithNamespace(ctx, es.cfg.Namespace)
	ctrEventSrv := es.client.EventService()

	if err := ctrEventSrv.Publish(namespaceCtx, topic, eventToPublish); err != nil {
		return fmt.Errorf("publishing event: %w", err)
	}

	return nil
}

// SubscribeTopic will subscribe to events on a named topic.
func (es *eventService) SubscribeTopic(ctx context.Context,
	topic string,
) (ch <-chan *ports.EventEnvelope, errs <-chan error) {
	topicFilter := topicFilter(topic)

	return es.subscribe(ctx, topicFilter)
}

// SubscribeTopics will subscribe to events on a set of named topics.
func (es *eventService) SubscribeTopics(ctx context.Context,
	topics []string,
) (ch <-chan *ports.EventEnvelope, errs <-chan error) {
	topicFilters := []string{}

	for _, topic := range topics {
		topicFilters = append(topicFilters, topicFilter(topic))
	}

	return es.subscribe(ctx, topicFilters...)
}

// Subscribe will subscribe to events on all topics.
func (es *eventService) Subscribe(ctx context.Context) (ch <-chan *ports.EventEnvelope, errs <-chan error) {
	return es.subscribe(ctx)
}

func (es *eventService) subscribe(ctx context.Context,
	filters ...string,
) (ch <-chan *ports.EventEnvelope, errs <-chan error) {
	var (
		evtCh     = make(chan *ports.EventEnvelope)
		evtErrCh  = make(chan error, 1)
		ctrEvents <-chan *events.Envelope
		ctrErrs   <-chan error
	)

	errs = evtErrCh
	ch = evtCh
	namespaceCtx := namespaces.WithNamespace(ctx, es.cfg.Namespace)

	if len(filters) == 0 {
		ctrEvents, ctrErrs = es.client.Subscribe(namespaceCtx)
	} else {
		ctrEvents, ctrErrs = es.client.Subscribe(namespaceCtx, filters...)
	}

	go func() {
		defer close(evtCh)

		for {
			select {
			case <-ctx.Done():
				if cerr := ctx.Err(); cerr != nil && !errors.Is(cerr, context.Canceled) {
					evtErrCh <- cerr
				}

				return
			case ctrEvt := <-ctrEvents:
				converted, err := convertCtrEventEnvelope(ctrEvt)
				if err != nil {
					evtErrCh <- fmt.Errorf("converting containerd event envelope: %w", err)
				}
				evtCh <- converted
			case ctrErr := <-ctrErrs:
				evtErrCh <- ctrErr
			}
		}
	}()

	return ch, errs
}

func topicFilter(topic string) string {
	return fmt.Sprintf("topic==\"%s\"", topic)
}
