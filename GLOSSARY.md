# flintlock

flintlock creates and manages microVMs on a host through a gRPC API, using
Firecracker or Cloud Hypervisor as the VMM.

## MicroVM status

**State**:
The lifecycle phase of a microVM's plan: `PENDING`, `CREATED`, `FAILED` or
`DELETING`. `CREATED` means the create plan ran; it says nothing about the
guest.
_Avoid_: phase, ready state

**Condition**:
One observed fact about a microVM, with a type, a status of `true`, `false` or
`unknown`, a reason, a message and the time its status last changed.
_Avoid_: health, readiness flag

**VMMRunning**:
The condition that the VMM process of a microVM runs.
_Avoid_: started, booted

**GuestAgentReady**:
The condition that the guest-agent inside a microVM answers a ping. It applies
only to a microVM that allows the guest-agent.
_Avoid_: guest ready, booted, usable
