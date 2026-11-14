package models

// EventEnvelope represents an event envelope.
type EventEnvelope struct {
	// ID is the unique identifier for the event.
	ID string `json:"id"`
	// Topic is the name of the topic the event originated from.
	Topic string `json:"topic"`
	// Event is the actual event payload.
	Event interface{} `json:"event"`
}
