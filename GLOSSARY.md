# flintlock

flintlock creates and manages microVMs on a host through a gRPC API. It runs
each microVM with a VMM (Firecracker or Cloud Hypervisor) and gives the guest
its cloud-init data.

## Language

**Spec metadata**:
The key-value map in a microVM spec that holds the cloud-init data of the
guest, with each value base64-encoded.
_Avoid_: Metadata (alone), instance metadata

**Vendor data**:
The cloud-init document that flintlock or its caller gives in addition to the
user data, under the key `vendor-data`.
_Avoid_: Vendor config

**Default vendor data**:
The empty cloud-config document that flintlock gives a Firecracker guest when
the spec metadata has no vendor data.
_Avoid_: Fallback vendor data, placeholder vendor data

**MMDS**:
The metadata service of Firecracker, from which a Firecracker guest reads its
cloud-init data over the network.
_Avoid_: Metadata server, IMDS

**Metadata interface**:
The network interface that flintlock adds to a Firecracker microVM so that the
guest can reach the MMDS.
_Avoid_: MMDS interface

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
