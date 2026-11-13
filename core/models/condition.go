package models

import "time"

// ConditionType is the type of a microvm condition.
type ConditionType string

const (
	// ConditionVMMRunning is the condition that the VMM process of the microvm runs.
	ConditionVMMRunning ConditionType = "VMMRunning"
	// ConditionGuestAgentReady is the condition that the guest-agent of the microvm answers a ping.
	ConditionGuestAgentReady ConditionType = "GuestAgentReady"
)

// ConditionStatus is the status of a microvm condition.
type ConditionStatus string

const (
	ConditionTrue    ConditionStatus = "True"
	ConditionFalse   ConditionStatus = "False"
	ConditionUnknown ConditionStatus = "Unknown"
)

const (
	ReasonRunning       = "Running"
	ReasonStartTimeout  = "StartTimeout"
	ReasonVMMExited     = "VMMExited"
	ReasonVMMStarting   = "VMMStarting"
	ReasonPingSucceeded = "PingSucceeded"
	ReasonPingTimeout   = "PingTimeout"
)

// Condition is one observed fact about a microvm.
type Condition struct {
	// Type is the type of the condition.
	Type ConditionType `json:"type"`
	// Status is the status of the condition.
	Status ConditionStatus `json:"status"`
	// Reason is a machine-readable reason for the status.
	Reason string `json:"reason"`
	// Message is a human-readable detail.
	Message string `json:"message"`
	// LastTransition is when the status last changed.
	LastTransition time.Time `json:"last_transition"`
}

// Conditions is the list of conditions of a microvm.
type Conditions []Condition

// Set adds or updates the condition with the type of cond. LastTransition
// changes to now only when the status changes.
func (c *Conditions) Set(cond Condition, now time.Time) {
	for i := range *c {
		existing := &(*c)[i]
		if existing.Type != cond.Type {
			continue
		}

		cond.LastTransition = existing.LastTransition
		if existing.Status != cond.Status {
			cond.LastTransition = now
		}

		*existing = cond

		return
	}

	cond.LastTransition = now
	*c = append(*c, cond)
}

// Get returns the condition with the type t.
func (c *Conditions) Get(t ConditionType) (Condition, bool) {
	for _, cond := range *c {
		if cond.Type == t {
			return cond, true
		}
	}

	return Condition{}, false
}
