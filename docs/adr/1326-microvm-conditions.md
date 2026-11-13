# 1326. MicroVM readiness is a list of conditions, not a new state value

- Status: accepted
- Date: 2026-10-14
- Authors: @richardcase
- Deciders: @richardcase
- ADR Discussion: https://github.com/liquidmetal-dev/flintlock/discussions/1325

## Context

A client of an agent sandbox must know when the guest of a microVM accepts
commands. Today it polls `GetMicroVM` until the state is `CREATED`, then polls
`ExecCommand` until the guest answers. The create plan also waits a fixed 5 s
after the start of the VMM. Open question 5 of the discussion asked how
flintlock reports that a microVM is ready.

## Decision

`MicroVMStatus` gets a list of conditions. Each condition has a `type`, a
`status` (`true`, `false` or `unknown`), a `reason`, a `message` and a
`last_transition` time. `last_transition` changes only when `status` changes.

There are two condition types to start:

- `VMMRunning`: the VMM process of the microVM runs.
- `GuestAgentReady`: the guest-agent answers a ping. It applies only to a
  microVM with `allow_guest_agent`.

`state` keeps its values. `CREATED` means that the create plan ran. A client
that needs the guest waits for `GuestAgentReady`.

Two options were rejected:

- A `READY` state value. It mixes the state of the VMM with the state of the
  guest, and each new signal would need another state value.
- A marker on the serial console. It depends on the image and on the serial
  log.

## Consequences

- The condition type is a string, so a new condition does not change the API.
- A `GuestAgentReady` of `false` does not fail the plan. The microVM is
  `CREATED` and the client decides what to do.
- The supervisor of #198 updates the conditions after the create.
