# 205. Isolation profile for untrusted guests

- Status: accepted
- Date: 2026-10-14
- Authors: @richardcase
- Deciders: @richardcase
- ADR Discussion: https://github.com/liquidmetal-dev/flintlock/discussions/1325

## Context

flintlock starts Firecracker directly, with `--no-api` and without the jailer (`infrastructure/microvm/firecracker/create.go`). Issue #205 asked for the jailer in 2021 and is open. The [production host setup guide](https://github.com/firecracker-microvm/firecracker/blob/main/docs/prod-host-setup.md) of Firecracker gives two options. Start Firecracker with the jailer binary of the release, or under process constraints at least as strict. The jailer gives a chroot, a uid and gid for each microVM, a network namespace, and a cgroup. Seccomp is on by default in Firecracker and is one layer of the three that its design document lists.

Cloud Hypervisor has no jailer. It runs as the user that starts it, and it has its own seccomp filters.

Discussion 1325 examines flintlock as a host for agent sandboxes. The code in a sandbox is untrusted. Requirement SBX-1 (hardware isolation) and SBX-8 (resource limits) ask for the layers that the jailer gives. Open question 2 of the discussion asked if flintlock must run the jailer for untrusted guests.

The Kubernetes node use case of flintlock does not need the jailer, and a change of the default breaks current deployments. The chroot of the jailer also changes each path that flintlock gives to the VMM: volumes, kernel, initrd, the vsock socket and the API socket.

## Decision

flintlock gets an isolation profile, set for the daemon with a flag such as `--isolation-profile`. The profile has two values:

| Value | Firecracker | Cloud Hypervisor |
| ----- | ----------- | ---------------- |
| `none` (default) | As today: Firecracker starts directly. | As today. |
| `jailer` | Firecracker starts through the jailer: a chroot under the state directory, a uid and gid for each microVM, a network namespace for each microVM (#206), and a cgroup for each microVM. | A network namespace for each microVM, a cgroup for each microVM, and a non-root user. There is no chroot. |

A host for untrusted guests sets `jailer`. The default stays `none` until the profile is stable in a release. A change of the default is a new ADR.

## Consequences

- #205 is the epic for the implementation. #206 (network namespace) is part of it.
- With `jailer`, each path that flintlock gives to Firecracker must resolve inside the chroot: volumes, kernel, initrd, the metadata file, the vsock socket and the API socket. The overlay boot of Option C and the snapshot work must plan for it. For snapshots, the chroot is the mechanism for SNAP-VOL-007 on Firecracker: a stored relative path resolves to the volume of the restored microVM.
- The egress policy of #1329 writes its rules in the network namespace that the profile creates.
- The e2e tests get a run with `jailer` for each provider, so the profile cannot rot.
- Issues and pull requests for the profile carry the `area/security` label.
