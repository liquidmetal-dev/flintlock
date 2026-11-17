---
weight: 8
title: Snapshots and clones
---

A snapshot holds the memory, the device state and the root disk of a running
microVM. You can restore a snapshot into a new microVM on the same host. The
new microVM continues from the point of the snapshot, so it skips the boot.
You can restore one snapshot more than once. Each restored microVM is a clone
of the source.

Snapshots are stored on the host that took them. This version cannot move a
snapshot to another host.

## What can be snapshotted

A microVM can be snapshotted when all of these are true:

- Its state is `CREATED`, and it is not being deleted.
- Its provider advertises the `snapshot` capability.
- Its root volume is an [`OVERLAY` volume](volumes.md#overlay-root).
- It has no additional volumes, no virtiofs volumes and no `FULL` volumes.
- It has `allow_guest_agent` set, and the guest agent is ready.
- No other snapshot of the microVM is in progress.

`CreateSnapshot` fails with `FAILED_PRECONDITION` when a rule is not met. The
error gives the reason.

A snapshot of a microVM with a macvtap interface can be restored only once.
The macvtap device is bound to the MAC address of the guest, so two restored
microVMs would share it. Firecracker has no macvtap support, so this applies to
Cloud Hypervisor.

## Take a snapshot

The examples use [grpcurl](https://github.com/fullstorydev/grpcurl) against
the gRPC API on `localhost:9090`. `flintlockd` serves gRPC reflection, so
grpcurl needs no proto files.

```bash
grpcurl -plaintext -d '{"source_uid": "<microvm-uid>", "name": "web-base"}' \
  localhost:9090 snapshot.services.api.v1alpha1.SnapshotService/CreateSnapshot
```

The name is optional. It must be unique in the namespace of the source
microVM. Without a name, flintlock uses `<source-name>-<yyyymmdd-hhmmss>` in
UTC. Each snapshot also has a uid that the server generates. The snapshot is
in the namespace of the source microVM.

`CreateSnapshot` returns at once, with the phase `PENDING`. The capture runs
in the background:

| Phase       | Meaning                                                                    |
| ----------- | -------------------------------------------------------------------------- |
| `PENDING`   | The snapshot is accepted and waits for a capture slot.                     |
| `CAPTURING` | The source microVM is paused, and its memory, state and disk are written.  |
| `READY`     | The snapshot is complete and can be restored. `local_path` is its directory. |
| `FAILED`    | The capture failed. `error` gives the reason.                              |

The API has other phases, such as `PACKAGING` and `PUSHING`, for a later
version. This version does not use them.

During the capture the source microVM is paused. It resumes when its memory
and disk are written, and it keeps running after the snapshot.

Get a snapshot by name and namespace, or by uid. grpcurl leaves out fields
with a zero value, and `PENDING` is the zero value of the phase. Use
`-emit-defaults` to show it:

```bash
grpcurl -plaintext -emit-defaults \
  -d '{"ref": {"named": {"namespace": "ns1", "name": "web-base"}}}' \
  localhost:9090 snapshot.services.api.v1alpha1.SnapshotService/GetSnapshot

grpcurl -plaintext -d '{"ref": {"uid": "<snapshot-uid>"}}' \
  localhost:9090 snapshot.services.api.v1alpha1.SnapshotService/GetSnapshot
```

List the snapshots of a namespace. You can filter by source microVM and by
labels:

```bash
grpcurl -plaintext -d '{"namespace": "ns1", "source_uid": "<microvm-uid>"}' \
  localhost:9090 snapshot.services.api.v1alpha1.SnapshotService/ListSnapshots
```

Delete a snapshot:

```bash
grpcurl -plaintext -d '{"ref": {"uid": "<snapshot-uid>"}}' \
  localhost:9090 snapshot.services.api.v1alpha1.SnapshotService/DeleteSnapshot
```

A snapshot that is `PENDING` or `CAPTURING` cannot be deleted. A deleted
snapshot is not returned by `GetSnapshot` or `ListSnapshots`. When a restored
microVM still uses the memory file, the files that it needs stay on the host.
They are removed when the last of those microVMs is deleted.

The HTTP API has the same calls:

| Call             | HTTP                                                    |
| ---------------- | ------------------------------------------------------- |
| `CreateSnapshot` | `POST /v1alpha1/snapshot`                               |
| `GetSnapshot`    | `GET /v1alpha1/snapshot/{uid}` or `GET /v1alpha1/snapshot/{namespace}/{name}` |
| `ListSnapshots`  | `GET /v1alpha1/snapshots/{namespace}`                   |
| `DeleteSnapshot` | `DELETE /v1alpha1/snapshot/{uid}`                       |

## Restore a snapshot

To restore a snapshot, call `CreateMicroVM` with `snapshot_source` in place
of `microvm`:

```bash
grpcurl -plaintext -d '{
  "snapshot_source": {
    "snapshot": {"named": {"namespace": "ns1", "name": "web-base"}},
    "name": "web-1"
  }
}' localhost:9090 microvm.services.api.v1alpha1.MicroVM/CreateMicroVM
```

The HTTP API has `POST /v1alpha1/microvm/restore`, with the `snapshot_source`
as the body.

The snapshot must be `READY`. The spec of the new microVM is the spec of the
source, with the changes that `snapshot_source` allows:

| Field               | Rule                                                                                    |
| ------------------- | --------------------------------------------------------------------------------------- |
| `snapshot`          | Required. A uid, or a name and a namespace.                                             |
| `name`              | Required. The name of the new microVM.                                                  |
| `namespace`         | Optional. The default is the namespace of the source.                                   |
| `uid`               | Optional. Generated when not set.                                                       |
| `labels`            | Optional. When empty, the labels of the source are used.                                |
| `metadata`          | Optional. When empty, the metadata of the source is used. Values are base64 encoded.    |
| `network_overrides` | Optional. Sets the host device of an interface, by its guest device name. Flintlock creates new host devices for the other interfaces. |
| `memory_mode`       | Optional. `FILE` on Firecracker and `COPY` on Cloud Hypervisor. These are the defaults, and each provider supports only its own. |
| `force`             | Optional. Skips some of the compatibility checks. See [Compatibility](#compatibility).  |

The vCPUs, the memory, the kernel, the initrd, the volumes and the network
interfaces always come from the source. A restore request cannot change them.

In `FILE` mode, the new microVM maps the memory file of the snapshot, and the
host loads its pages when the guest uses them. In `COPY` mode, the VMM reads
the whole memory file when it starts.

Each restored microVM gets its own copy of the root disk. Writes in one
restored microVM do not change the snapshot, the source or another restored
microVM. The read-only base of the overlay root is shared.

The new microVM is `CREATED` when its VMM runs and its guest agent is ready.
From then on it is an ordinary microVM. If its VMM stops, flintlock boots it
again from its own disk, not from the snapshot.

The base of the snapshot must still be on the host. The snapshot keeps it
from being removed. If it is missing, the restore fails.

## What a restore keeps and changes

The guest continues from the point of the snapshot:

- The processes, the memory and the uptime continue.
- The guest clock starts at the time of the snapshot. It jumps forward when
  the guest syncs it, for example with NTP.
- The vsock device is reset. Open vsock connections close, and the guest
  agent listens again.
- Open TCP connections usually break. The peers have timed out, or they do
  not expect the restored guest.
- On Firecracker, flintlock puts the MMDS metadata again before the guest
  resumes, so the guest reads the metadata of the new microVM.

Flintlock adds three keys to the metadata of a restored microVM:

| Key                            | Value                                        |
| ------------------------------ | -------------------------------------------- |
| `flintlock.snapshot.uid`       | The uid of the snapshot.                     |
| `flintlock.restore.uid`        | The uid of the restored microVM.             |
| `flintlock.restore.generation` | A number that is different for each restore of the snapshot. The first restore gets `1`. |

The guest can read these keys to find out that it was restored, and from
which snapshot.

## Clones

Flintlock does not change anything inside the guest. All the clones of one
snapshot start with the same:

- State of the random number generator.
- Secrets in memory and on disk, such as SSH host keys, tokens and session
  keys.
- Hostname, machine ID, and the MAC and IP addresses in the guest.

Do one or more of these to make clones safe:

- Use a guest kernel with VMGenID. Firecracker gives the guest a new
  generation ID at each restore. The kernel then reseeds its random number
  generator. This needs Linux 5.18 or later on x86_64. The 6.1 kernels of
  [mikrolite-images](https://github.com/liquidmetal-dev/mikrolite-images) have it.
  On arm64, Firecracker needs a 6.1 kernel with the DeviceTree binding that
  was added in Linux 6.10.
- Read the restore keys in the guest. When `flintlock.restore.uid` changes,
  make new secrets and a new machine ID, and set a new hostname and new
  addresses.
- Put the clones on separate networks, so their MAC and IP addresses do not
  clash.

## Compatibility

A restore compares the snapshot with the host. A mismatch fails the restore
with `FAILED_PRECONDITION`, and the error names each field with both values.

| Field                                       | Checked                 |
| ------------------------------------------- | ----------------------- |
| VMM type, CPU architecture, CPU vendor      | Always                  |
| Snapshot format version (Firecracker)       | Always                  |
| VMM version (Cloud Hypervisor)              | Always                  |
| VMM version (Firecracker)                   | Unless `force` is set   |
| Host kernel, CPU model, CPU microcode       | Unless `force` is set   |

Use `force` only when you know that the change does not affect the guest.

## Files on the host

The snapshots are in `snapshots/` in the state directory (`--state-dir`,
default `/var/lib/flintlock`). Each snapshot has a directory named by its uid:

| File              | Content                                                       |
| ----------------- | ------------------------------------------------------------- |
| `record.json`     | The name, the source, the labels and the phase.               |
| `descriptor.json` | The facts of the host and the microVM, for the compatibility check. |
| `spec.json`       | The spec of the source microVM, with its metadata.            |
| `vmstate`         | The state of the VMM. A directory on Cloud Hypervisor.        |
| `memory`          | The memory of the guest (Firecracker).                        |
| `root.rw`         | A copy of the writable disk of the source.                    |
| `root.base`       | A link to the read-only base.                                 |
| `refs/`           | One empty file for each restored microVM that maps `memory`.  |

`root.rw` is a reflink copy, so it uses disk space only for the blocks that
change later. The memory file is as large as the memory of the microVM.

## Security

Snapshots are not encrypted. The memory file and `spec.json` hold the secrets
of the guest, and the metadata of the source.

The snapshot directories have mode `0700`, and the files have mode `0600`.
Give access to the state directory only to the user that runs `flintlockd`.

## Daemon flags

`flintlockd` has these flags for snapshots:

| Flag                        | Default | Description                                                                    |
| --------------------------- | ------- | ------------------------------------------------------------------------------ |
| `--snapshot-max-pause`      | `10s`   | The longest time a capture keeps the source paused. When it runs out, the source resumes and the snapshot fails. |
| `--snapshot-max-concurrent` | `1`     | The number of captures that run at the same time. Each capture writes the whole memory of its source. |
