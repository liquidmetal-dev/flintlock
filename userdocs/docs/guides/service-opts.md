---
title: Flintlockd options
---

_A full reference is coming soon..._

## Directories

| Flag           | Default              | Description                                                        |
| -------------- | -------------------- | ------------------------------------------------------------------ |
| `--state-dir`  | `/var/lib/flintlock` | Root directory for microvm runtime state (config, logs, pid files). |
| `--socket-dir` | `/run/flintlock`     | Root directory for per-microvm unix sockets.                       |

Each microvm's unix sockets (the Cloud Hypervisor API socket, the guest-agent vsock
socket and the virtiofs socket) are created in `<socket-dir>/<microvm uid>/`. The
path is keyed only by the microvm's UID so that it stays within the Linux unix socket
path limit (107 characters) whatever the microvm's namespace and name.

`--socket-dir` must be an absolute path. `flintlockd` won't start if it's too long
for the socket paths to fit within the limit, and the error states the maximum length.

:::note
Microvms created by an earlier version of flintlock have their sockets in the state
directory. They keep working until they're next re-created, which moves their sockets
to the socket directory. This fallback will be removed in the next minor release.
:::
