---
title: Volumes
---

A microVM has one root volume and any number of additional volumes. A volume
comes from an OCI image (`container_source`) or from a directory on the host
(`virtiofs_source`). This page covers the fields that select how a volume
from an OCI image is provisioned.

## Volume mode

The `mode` field of a volume selects how `flintlockd` provisions it.

| Mode      | What the host does                                                                                                                                 |
| --------- | -------------------------------------------------------------------------------------------------------------------------------------------------- |
| `FULL`    | Unpacks the image into a containerd snapshot and attaches it as one block device. This is the default and the only mode before `mode` existed.   |
| `OVERLAY` | Builds one read-only base per image and shares it between microVMs. Each microVM gets its own writable disk. The guest's init joins them with overlayfs. |

A request that does not set `mode` gets `FULL`. Clients written before the
field existed keep working.

Only the root volume can use `OVERLAY`. Additional volumes from an OCI image
are always `FULL`.

### Size in overlay mode

In `OVERLAY` mode `size_in_mb` is the size of the writable disk. The minimum is
64 MB. Leave it unset to use the daemon default.

### Rules

`flintlockd` rejects a request when:

- The root volume is `OVERLAY` and `is_read_only` is set. An overlay root needs
  its writable disk.
- An additional volume is `OVERLAY`.
- The root volume is `OVERLAY` and `size_in_mb` is set below 64.
- The root volume is `OVERLAY` and the kernel command line sets `rdinit`. The
  overlay initramfs `/init` is the contract between flintlock and the guest,
  and `rdinit` would replace it.

## Capabilities

A provider advertises the volume modes its host can serve:

| Capability       | Accepts        | Host needs                                                                    |
| ---------------- | -------------- | ----------------------------------------------------------------------------- |
| `volume-full`    | `FULL` volumes | The containerd devmapper snapshotter (a thin pool).                           |
| `volume-overlay` | `OVERLAY` root | A state directory on a filesystem with reflinks, `mkfs.erofs` and `mkfs.ext4`. |

A request for a mode the provider does not advertise fails with an error
that names the missing capability. Today both providers advertise
`volume-full`. No provider advertises `volume-overlay` yet, so a request with
`mode: OVERLAY` is rejected.

## Volume status

The status of a volume has a mount for each file or device the host attaches.

| Field         | `FULL`                     | `OVERLAY`                                        |
| ------------- | -------------------------- | ------------------------------------------------ |
| `mount`       | The block device (`DEV`).  | The writable disk (`FILE`).                      |
| `base_mount`  | Unset.                     | The read-only base (`FILE`).                     |
| `base_digest` | Empty.                     | The sha256 digest of the base.                   |

A mount of type `HOSTPATH` is a directory on the host. It is used for kernel
and initrd mounts and for virtiofs volumes.
