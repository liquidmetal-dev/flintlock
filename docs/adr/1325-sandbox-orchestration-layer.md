# 1325. Sandbox orchestration lives above flintlock

- Status: accepted
- Date: 2026-10-14
- Authors: @richardcase
- Deciders: @richardcase
- ADR Discussion: https://github.com/liquidmetal-dev/flintlock/discussions/1325

## Context

Discussion 1325 examines flintlock as a host for agent sandboxes. Three of its requirements ask for resources that flintlock does not have: templates (SBX-10), sessions with persistence and fork (SBX-11), and placement across a fleet of hosts (SBX-13). Sandbox products give these to their users as a session and template API above the VM. Open question 1 of the discussion asked if that API is in scope for flintlock, or for a service above it.

Two services in the liquidmetal-dev organization already give most of that layer:

- [Battery](https://github.com/liquidmetal-dev/battery) is a warm pool manager for flintlock. Its `PoolAdmin` service holds pool definitions, which are templates. Its `Lease` service claims a microVM from a pool, sends heartbeats and releases it, which is a session. Its host agent runs next to each `flintlockd` and proxies the guest-agent `exec` and `ping` operations over gRPC.
- [Brigade](https://github.com/liquidmetal-dev/brigade) is a distributed orchestrator. It implements the `MicroVM` gRPC service of flintlock as a drop-in. It selects a host for each microVM and forwards the request to the `flintlockd` of that host.

flintlock was designed as a service for one host. It has no placement, no pools and no sessions.

## Decision

flintlock exposes microVM primitives for one host only: create, get, list, delete, pause, resume, stop (#1330), snapshot and restore (#204), exec, the ssh proxy, and file put and get (#1332).

flintlock does not get a template, session, pool or placement resource. Those live above it:

| Concept | Where | Resource |
| ------- | ----- | -------- |
| Template | Battery | pool definition |
| Session | Battery | lease |
| Warm pool | Battery | pool |
| Placement across hosts | Brigade | scheduler |
| Fork of a session | flintlock snapshot and restore, driven by Battery | snapshot package |

## Consequences

- Each new RPC in flintlock must be simple to forward. Brigade vendors the `MicroVM` service and forwards each request to one host. Exec and file transfer need the vsock socket of a host. The host agent of Battery, or Brigade, must proxy such an RPC.
- Issues filed for the layer above: liquidmetal-dev/battery#129 (pause of an idle lease), liquidmetal-dev/battery#130 (file proxy through the host agent), liquidmetal-dev/brigade#24 (pass-through of pause, resume and stop), liquidmetal-dev/brigade#25 (proxy of the `MicroVMExec` service).
- A product that needs a sandbox API builds on Battery and Brigade, not on flintlock alone. The flintlock issues from the discussion (#1326 to #1333) stay at the host level.
- This decision does not block a future flintlock resource. A new ADR supersedes this one if the layer above is not enough.
