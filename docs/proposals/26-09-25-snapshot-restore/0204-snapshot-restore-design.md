# 0204. Snapshot, Restore and Clone: design for the second Option C milestone

* Status: proposed design
* Date: 2026-11-13
* Authors: @phoban01
* Issue: [#204](https://github.com/liquidmetal-dev/flintlock/issues/204)
* Companions: [requirements](0204-snapshot-restore-requirements.md),
  [overlay boot design](0204-overlay-boot-design.md),
  [decision record](0204-root-volume-decision.md),
  [Option C](0204-option-c-ro-base-rw-disk.md),
  [ADR 0205, isolation profile](../../adr/0205-isolation-profile.md)

## 1. Summary

This document designs the second milestone of Option C. A client takes a
snapshot of a running microVM whose root is an `overlay` volume. It then
restores that snapshot on the same host, as often as it likes. Each restored
microVM resumes where the source was, with its own writable disk, TAP
devices and sockets. All restored microVMs map one memory file and share one
base.

The milestone stays on one host. Push and pull of a snapshot package, and
restore on another host, come later. This milestone gives the capture, the
restore and the clone path that the cross-host work is built on.

The open questions were settled in the issues of epic
[#1358](https://github.com/liquidmetal-dev/flintlock/issues/1358). This
document records those twelve decisions and the reason for each. Two spikes
check the Firecracker behaviour that decisions 3, 4 and 8 rest on
([section 9](#9-spikes)).

## 2. Relationship to other documents

The design builds on the [overlay boot design](0204-overlay-boot-design.md),
the first milestone. That milestone gave the layout this one captures: a
read-only EROFS base shared by many microVMs and a writable ext4 disk per
microVM, both on one reflink filesystem. Where this document departs from the
overlay boot design or from the requirements, the change is listed as a
numbered amendment in [section 11](#11-amendments).

The requirements are those of sections 5.1 to 5.8, 5.10 and 5.11 of the
[requirements](0204-snapshot-restore-requirements.md), limited to the scope
of #1358 ([section 3](#3-scope)). A decision cites the requirement IDs it
satisfies.

Each decision has a number from issue
[#1359](https://github.com/liquidmetal-dev/flintlock/issues/1359). The issue
named in the table holds the detail.

| # | Decision | Issue | Section |
| - | -------- | ----- | ------- |
| 1 | Local store: a plain directory per snapshot, not an OCI layout | #1372 | [4.2](#42-local-store) |
| 2 | Snapshot record: files, a flock and a startup scan; no database | #1372 | [4.3](#43-snapshot-record) |
| 3 | Drive paths of a Firecracker clone: relative, against a working directory per microVM | #1362 | [6.1](#61-drive-paths-of-a-firecracker-clone) |
| 4 | Vsock of a clone: relative, through the `sock` link; API socket on the command line | #1370, #1360 | [6.2](#62-sockets-of-a-firecracker-clone) |
| 5 | CPU configuration deferred; the compatibility check is for one host | #1371 | [5.4](#54-host-facts-and-the-compatibility-check) |
| 6 | Generation counter: an integer in `record.json`, incremented under the flock | #1372 | [4.4](#44-generation-counter) |
| 7 | MMDS data is put again after a load, with the restore markers | #1362, #1365 | [6.4](#64-firecracker-restore) |
| 8 | Memory file reference: `refs/<vm-uid>` files | #1372 | [4.5](#45-references-to-the-memory-file) |
| 9 | Startup recovery of an interrupted capture | #1374 | [7.2](#72-startup-recovery) |
| 10 | The `snapshot` capability and the eligibility rules | #1373 | [6.6](#66-capability-and-eligibility) |
| 11 | A snapshot controller, a per-microVM lock, and two limits | #1374 | [6.8](#68-snapshot-controller) |
| 12 | Identity of a snapshot, the restore request, and the cold boot after a crash | #1361, #1365 | [5.5](#55-api), [6.9](#69-restore), [6.11](#611-cold-boot-after-a-crash) |

## 3. Scope

### 3.1 In scope

| Area | What is built | Requirements |
| ---- | ------------- | ------------ |
| Provider port | Pause and resume, with the Firecracker API socket | SNAP-PRV-001, 002, 006 |
| Capture | Pause, VMM snapshot, reflink of the writable disk, resume | SNAP-CRT, SNAP-VOL-001 to 008 and 012 |
| Local store | A snapshot record with the compatibility descriptor and the source spec | SNAP-META-001 to 005, SNAP-OPS-005 |
| API | `SnapshotService`, and a snapshot source in `CreateMicroVM` | SNAP-API-001 to 006 |
| Restore | Restore markers, the compatibility check against this host, the retained memory file | SNAP-RST |
| Clones | One snapshot restored more than once on one host | SNAP-CLN-001 to 006 |
| Operations | Events, clean-up after a failure, recovery after a restart | SNAP-OPS-001, 003, 004 |

Firecracker comes first. Cloud Hypervisor has its own issues (#1376, #1377,
#1378), off the critical path. Its `snapshot` capability stays off until
#1377 is merged.

### 3.2 Out of scope

* Push and pull of a package, and restore on another host (SNAP-PKG-001 to
  010). SNAP-RST-004 and 005 apply only as a check against the same host.
* The OCI image layout of the local store (SNAP-PKG-011). Decision 1 says
  what the move costs later.
* The effective guest-visible CPU configuration (open question 9).
* Quiesce (SNAP-QSC). The guest agent has no freeze and thaw yet.
* Stop of the source after the snapshot (SNAP-CRT-006). It needs the
  stopped state of #1330. The source is always resumed.
* Restore policy and signatures (SNAP-SEC-002 to 005).
* `Uffd` and the lazy Cloud Hypervisor memory modes (SNAP-RST-009).
* `ServerInfo` fields (SNAP-API-007) and metrics (SNAP-OPS-002).
* A `full` root, additional volumes (#1379, a later milestone) and virtiofs
  (SNAP-CRT-004, SNAP-VOL-010).
* More than one restore of a microVM with a macvtap interface
  (SNAP-CLN-004).
* arm64 (#1317). The milestone is amd64.

### 3.3 Success criteria

1. A client takes a snapshot of a running overlay microVM on Firecracker.
   The source keeps running, and the guest stalls for less than
   `--snapshot-max-pause`.
2. The snapshot restores twice on the same host. Each restored microVM
   continues the source's processes, files and uptime.
3. The metadata of each restored microVM has the restore markers, with
   generations 1 and 2.
4. The source and the restored microVMs have distinct writable disks, TAP
   devices and sockets. They share the base and the memory file. A write in
   one is visible in no other.
5. A restore that changes the vCPU count, or that targets an incompatible
   host, is refused with an error that names the field.
6. A deleted snapshot keeps its memory file while a restored microVM maps it.
   The file goes when the last such microVM is deleted.
7. After a restart of `flintlockd` during a capture, the snapshot is
   `failed` and the source runs.
8. Existing `v1alpha1` clients and the existing e2e suite work with no edit
   (SNAP-API-004).

## 4. Host layout

### 4.1 On-host layout

```text
<StateRootDir>/                    # --state-dir, default /var/lib/flintlock
├── vm/<ns>/<name>/<uid>/          # state dir, working dir of the VMM
│   ├── sock -> <SocketDir>/<uid>  # #1370
│   └── volumes/
│       ├── root.base -> ../../../../../bases/<sha256>.img
│       └── root.rw
├── bases/<sha256>.img
└── snapshots/<uuid>/              # 0700, one per snapshot
    ├── record.json                # 0600, the record
    ├── descriptor.json            # 0600, compatibility descriptor
    ├── spec.json                  # 0600, source spec with metadata
    ├── vmstate                    # 0600, VMM state (a dir on Cloud Hypervisor)
    ├── memory                     # 0600, guest memory (Firecracker)
    ├── root.rw                    # 0600, reflink of the source's disk
    ├── root.base -> ../../bases/<sha256>.img
    ├── refs/<vm-uid>              # one empty file per FILE restore
    └── .lock                      # flock for record.json and refs/

<SocketDir>/<uid>/                 # --socket-dir, default /run/flintlock
├── firecracker.sock               # API socket (#1360)
└── guest-agent.vsock              # vsock socket
```

Everything under the state root is on one filesystem, as in the overlay boot
design, so that reflinks and renames work across `vm/`, `bases/` and
`snapshots/`.

The rule of the overlay boot design still holds. Every fact about a snapshot
is derivable from this layout. There is no ledger and no database.

### 4.2 Local store

Decision 1. The local store is a plain directory per snapshot, with a
`record.json`. It is not an OCI image layout (#1372, open question 1).

The reasons:

* This milestone has no push or pull, so nothing reads an OCI layout yet.
* An OCI layout names each blob by its sha256. The capture would have to
  hash the memory file and the writable disk, which are gigabytes. That
  delays `ready` and gives nothing on one host.
* A Firecracker `File` restore maps the memory file by its path. A packaged
  memory layer is zstd-compressed (SNAP-PKG-004), so it cannot be mapped.
  The uncompressed file has to exist outside the layout in any case.

What the move to an OCI layout costs in the push and pull milestone
(SNAP-PKG-011):

* A packaging step after `ready`. It reads `memory` and `root.rw` once,
  compresses the memory with zstd, splits it into layers (SNAP-PKG-005),
  hashes every blob, and writes `blobs/sha256/`, `index.json` and
  `oci-layout`.
* The descriptor becomes the config blob. `spec.json` becomes a layer. Both
  keep their content, so the reader of #1371 does not change.
* Disk space. A packaged snapshot holds both the plain files and the
  compressed blobs until the plain files can go. On one host they cannot go
  while a `File` restore maps `memory`. Reflinks do not help, because the
  compressed blob has other content.
* The volume layer. This milestone keeps the whole writable disk as a
  reflink. A package carries a delta against the base, or a full image
  (SNAP-VOL-009). The delta encoding (open question 6) is new work.
* A pulled package has to be unpacked into the plain layout before a `File`
  restore. So the plain directory stays as the form a restore runs from, and
  the OCI layout is the form a push and pull move.
* No migration. A snapshot taken by this milestone is packaged the first
  time it is pushed. `record.json`, `refs/` and the `SnapshotRepository`
  port stay as they are.

### 4.3 Snapshot record

Decision 2. The record is files in the snapshot directory, protected by a
flock, and rebuilt by a scan at startup. There is no database (#1372).

The record in `record.json` has: `uid` (a UUID), `name`, `namespace` (the
source's), `source` (uid, name, namespace), `labels`, `phase` (`pending`,
`capturing`, `ready`, `failed`), `error`, `created_at`, `generation` and
`deleted`. The references are the files in `refs/`, not a field.

The store writes `record.json` to a temporary file and renames it. Every
change to `record.json` or to `refs/` holds the flock on `.lock`. A
store-wide flock covers the name check in `Create`, so two creates with one
name cannot both pass. At startup the store scans `snapshots/` and removes a
directory that has no `record.json`. That is a create that stopped before
the record was written.

The reasons:

* Amendment A7 of the overlay boot design moved records into the SQLite
  store of [#1251](https://github.com/liquidmetal-dev/flintlock/pull/1251).
  That pull request is not merged. This milestone does not wait for it.
* The locks are flocks, not in-process mutexes.
  `internal/command/run/run.go` starts two sets of ports, one for the API
  server and one for the controllers. Both write to the store.
* A scan can rebuild everything from the files. A database added later is an
  index over them, as the overlay boot design requires.

The record satisfies SNAP-OPS-005 (records persist) and SNAP-SEC-001 (files
0600, directories 0700).

### 4.4 Generation counter

Decision 6. The restore generation is an integer `generation` in
`record.json` (#1372, open question 3). It starts at 0.
`SnapshotRepository.NextGeneration` increments it under the flock and returns
the new value, so the first restore gets 1.

The reasons:

* It is the simplest value that meets SNAP-RST-008 on one host. Under the
  flock, no two restores of one snapshot get the same value.
* The value lives with the snapshot and survives a restart.
* Who allocates it across hosts is a question for the cross-host milestone.
  A pulled package has no counter of the source host, so that milestone has
  to choose a new scheme.

A restore that fails after `NextGeneration` leaves a gap. Generations are
unique, not contiguous.

### 4.5 References to the memory file

Decision 8. A restored microVM that maps the memory file (`FILE` mode) has
one empty file `refs/<vm-uid>` in the snapshot directory (#1372). The restore
plan calls `AddRef` and `DeleteMicroVM` calls `ReleaseRef`.

`DeleteSnapshot` of a snapshot with no refs removes its directory. With refs,
it sets `deleted`, keeps `record.json`, `vmstate`, `memory` and `refs/`, and
removes the rest. A deleted record is not returned by `Get` or `List`. The
last `ReleaseRef` of a deleted record removes the directory.

The reasons:

* SNAP-RST-010 asks for a containerd lease or an equivalent. The memory file
  is not in the containerd content store, so a lease does not apply.
* A file per microVM is idempotent. A plan step that runs twice creates the
  same file, and a delete that runs twice removes nothing more. A counter in
  `record.json` would count a retried step twice.
* The references survive a restart and can be checked against the microVMs
  that exist.

This satisfies SNAP-API-005 and SNAP-API-006. A `COPY` restore reads the
memory once at start, so it takes no reference.

The `root.base` link in the snapshot directory is the reference from the
snapshot to its base (#1363). The sweep of `infrastructure/volume/overlay`
counts `snapshots/*/root.base` links as references, as it counts the
`volumes/root.base` links of microVMs (amendment S2).

## 5. Ports

### 5.1 Provider port

```go
// core/ports/services.go, MicroVMService
// Pause pauses a running microvm. (#1360, merged)
Pause(ctx context.Context, vm *models.MicroVM) error
// Resume resumes a paused microvm. (#1360, merged)
Resume(ctx context.Context, vm *models.MicroVM) error
// Snapshot writes dest/vmstate and dest/memory. The caller has paused vm. (#1362)
Snapshot(ctx context.Context, vm *models.MicroVM, dest string) error
// Restore starts the VMM of vm from the snapshot in src and leaves it
// running. The caller has made the network interfaces and volumes. (#1362)
Restore(ctx context.Context, vm *models.MicroVM, src string, in RestoreInput) error
```

`RestoreInput` carries the `MemoryMode`. `MicroVMStatePaused` is a provider
state (#1360). The create and update plan treats a paused microVM as
running, so no step runs for it.

### 5.2 Volume port

```go
// core/ports/services.go, VolumeService (#1363)
// Capture reflinks volumes/root.rw of ref to destDir/root.rw and links
// destDir/root.base to the base. It is idempotent.
Capture(ctx context.Context, ref VolumeRef, destDir string) (*models.CapturedVolume, error)
// Restore reflinks SnapshotDir/root.rw to the new microvm's volumes/root.rw
// and links volumes/root.base to the snapshot's base.
Restore(ctx context.Context, in VolumeRestoreInput) (*VolumeMounts, error)
```

`CapturedVolume` returns the base key, the base digest, the size and the
block size (from `statfs`). Both calls use `FICLONE` through `pkg/reflink`.
A capture of a 20 GB disk pauses the guest for milliseconds, and a restore
copies nothing (SNAP-VOL-004). `FICLONE` not supported is a configuration
error, as in `Create`. `Release` of a restored microVM removes its own disk
and links, and does not touch the snapshot directory.

### 5.3 Snapshot repository

```go
// core/ports/repositories.go (#1372)
type SnapshotRepository interface {
    Create(ctx context.Context, s *models.Snapshot) error
    Get(ctx context.Context, ref models.SnapshotRef) (*models.Snapshot, error)
    List(ctx context.Context, filter SnapshotFilter) ([]*models.Snapshot, error)
    Update(ctx context.Context, s *models.Snapshot) error
    Delete(ctx context.Context, uid string) error
    Dir(uid string) string
    NextGeneration(ctx context.Context, uid string) (int, error)
    AddRef(ctx context.Context, uid, vmUID string) error
    ReleaseRef(ctx context.Context, uid, vmUID string) error
}
```

The implementation is in `infrastructure/snapshot/local`. `Create` returns
`AlreadyExists` on a name clash.

### 5.4 Host facts and the compatibility check

Decision 5. The compatibility check is for one host. The effective
guest-visible CPU configuration is not collected (#1371, open question 9).

`HostInfoService.Facts(ctx, provider)` (in `infrastructure/hostinfo`, merged
in #1388) returns the facts of the host. The capture plan writes them, with
the facts of the microVM, into `descriptor.json`
(`core/models/snapshot_descriptor.go`, `format_version` 1).

`CheckCompatibility(desc, facts, force)` in `core/application/compat.go`
compares a descriptor with the host:

| Field | Enforced | Requirement |
| ----- | -------- | ----------- |
| `vmm_type`, `arch`, `cpu_vendor` | always | SNAP-RST-004 |
| `fc_snapshot_version` (Firecracker) | always | SNAP-RST-004 |
| `vmm_version` (Cloud Hypervisor) | always | SNAP-RST-004 |
| `host_kernel`, `cpu_model`, `cpu_microcode` | unless `force` | SNAP-RST-005 |
| `vmm_version` (Firecracker) | unless `force` | SNAP-RST-005 |

The error is a `CompatibilityError` that names each field with both values.
The descriptor also records `cpu_template`. A restored microVM takes the
source's spec, so its CPU template is the source's by construction.

The reasons:

* The milestone restores on the same host only. On one host, the CPU
  configuration a microVM sees changes only when the host kernel, the
  microcode, the CPU or the VMM changes. The check compares each of those.
* `cpu-template-helper fingerprint dump` boots a throwaway microVM. That is
  a cost per host, CPU template, kernel and Firecracker version, with a
  cache to invalidate. It buys nothing on one host.
* The cross-host milestone adds the field and fills it from that tool,
  cached per host. The descriptor has a `format_version` for that change.

The descriptor has no field for the `metadata` map (SNAP-META-004). The
metadata is in `spec.json` (SNAP-META-003). Flintlock reads the descriptor
without reading the memory, the VMM state or the disk (SNAP-META-002).

### 5.5 API

Decision 12, first part: identity and the restore request (#1361, merged in
#1385).

A snapshot has a UUID that the server generates, and an optional name. The
name is unique in its namespace among records that are not deleted. The
namespace is the source microVM's. Without a name, the store gives
`<source-name>-<yyyymmdd-hhmmss>` in UTC. A `SnapshotRef` is a `oneof` of
`uid` or of `name` and `namespace`.

The reasons:

* A microVM has the same identity: a uid, and a name in a namespace. Clients
  already work with that shape.
* A name gives a client a stable handle, for example the snapshot a battery
  pool refills from (ADR 1325).
* The namespace of the source keeps a snapshot with the tenant it came from.

`SnapshotService` in `snapshot.services.api.v1alpha1` has `CreateSnapshot`,
`GetSnapshot`, `ListSnapshots`, `ListSnapshotsStream` and `DeleteSnapshot`,
with REST mappings as `microvms.proto` (SNAP-API-001). `CreateSnapshot`
returns at once with a `pending` snapshot; the client polls `GetSnapshot` or
watches the events (SNAP-CRT-008). `SnapshotStatus` has the full phase enum
of SNAP-API-002. This milestone uses `PENDING`, `CAPTURING`, `READY` and
`FAILED`. `local_path` is the snapshot directory when `READY`.

`After` declares `RESUME` and `STOP`. Only `RESUME` is accepted until #1330.

The restore request is a `SnapshotSource` in `CreateMicroVMRequest`
(SNAP-API-003). `POST /v1alpha1/microvm` keeps its body, and
`POST /v1alpha1/microvm/restore` takes a `snapshot_source` body
(SNAP-API-004).

| Field of `SnapshotSource` | Rule |
| ------------------------- | ---- |
| `snapshot` | Required. Resolved to the snapshot uid once (section 6.9) |
| `name` | Required |
| `namespace` | Defaults to the source's |
| `uid` | Generated unless given |
| `labels`, `metadata` | Empty means the source's (SNAP-RST-007) |
| `network_overrides` | Set host device names by guest device name. Other TAPs are generated |
| `memory_mode` | `FILE` (Firecracker default) or `COPY` (Cloud Hypervisor default) |
| `force` | Skips the SNAP-RST-005 checks only |

vCPUs, memory, kernel, initrd, volumes and the set and order of interfaces
are always the source's. A request cannot change them (SNAP-RST-003).

## 6. Providers and plans

### 6.1 Drive paths of a Firecracker clone

Decision 3. A Firecracker clone finds its own drives through relative paths.
Each path resolves against the working directory of the process, which is
the state directory of the microVM (`cmd.Dir` in `create.go`) (#1362, open
question 8).

The overlay drives are already relative: `volumes/root.base` and
`volumes/root.rw` (`infrastructure/microvm/shared/overlay.go`). Firecracker
stores the path string in the snapshot. A restored process with its own
working directory opens its own `volumes/` (SNAP-VOL-007, SNAP-CLN-002).

The reasons:

* Firecracker has no drive override at snapshot load. It reopens each drive
  at the path the snapshot holds.
* The first milestone made overlay drives relative for this reason, so no
  path changes now.
* Only a microVM with an overlay root and no additional volumes can be
  snapshotted (decision 10). So no absolute drive path is ever in a
  snapshot.
* It needs no new process isolation. The other ways, a mount namespace per
  microVM or the jailer, are larger changes.

The jailer chroot of
[ADR 0205](../../adr/0205-isolation-profile.md) is the same mechanism later.
Under the `jailer` profile, the chroot of the microVM is its root, and the
same relative strings resolve inside it. ADR 0205 names the chroot as the
mechanism for SNAP-VOL-007. A move to it changes where the paths resolve,
not what the snapshot stores.

Spike A checks this on Firecracker v1.16.1. If it fails, the fallback of
[section 9.3](#93-fallback) applies.

### 6.2 Sockets of a Firecracker clone

Decision 4. The vsock path in the Firecracker configuration is the relative
`sock/guest-agent.vsock` (#1370, merged in #1387). `ensureState` creates the
link `<state dir>/sock` to the socket directory `<SocketDir>/<uid>`, mode
0700. One function, `RelativeVSockPath` in
`infrastructure/microvm/firecracker/state.go`, gives the path.

The reasons:

* Firecracker stores `vsock.uds_path` in the snapshot and has no override
  for it at load. An absolute path would make a clone bind the source's
  socket.
* The socket file stays at `<SocketDir>/<uid>/guest-agent.vsock`, so the
  guest-agent dialer and the stale-socket clean-up do not change.
* The socket stays under `/run/flintlock`, not in the state directory. The
  state directory path is long, and a Unix socket path has a limit of 107
  bytes (`models.MaxUnixSocketPathLength`).
* One function changes for the jailer profile later.

This applies to every Firecracker microVM, not only to snapshot sources. A
microVM created before #1370 keeps its absolute path until it is recreated.

The API socket is `<SocketDir>/<uid>/firecracker.sock`, passed with
`--api-sock` on the command line (#1360, merged in #1386). It replaces
`--no-api` (SNAP-PRV-001). The command line is not part of a snapshot, so a
restored process binds its own API socket with no extra work.

Firecracker resets the vsock device on a snapshot. Guest-agent connections
close, and the agent in the restored guest binds again. The restore plan
waits for `GuestAgentReady` for this reason.

### 6.3 Firecracker snapshot

`Snapshot(vm, dest)` sends `PUT /snapshot/create` with:

| Field | Value |
| ----- | ----- |
| `snapshot_type` | `Full` (SNAP-CRT-009) |
| `snapshot_path` | `dest/vmstate` |
| `mem_file_path` | `dest/memory` |

The body has no `sync_snapshot_files`: spike A shows that v1.16.1 refuses
the field (section 9.4). The provider calls `fsync` on both files after
the call (SNAP-PRV-005, amendment S7). Then it sets both files to 0600
(SNAP-SEC-001). The calls go through `pkg/firecracker`, the client added by
#1360.

### 6.4 Firecracker restore

`Restore(vm, src, in)`:

1. `ensureState` for the new microVM: the state directory, the `sock` link
   and the removal of stale sockets.
2. Start `firecracker --id <uid> --api-sock <sock> --boot-timer` with
   `cmd.Dir` set to the state directory. There is no `--config-file` and no
   `--metadata`. The pid, log and metrics handling is that of `create.go`.
   Wait until the API socket answers.
3. `PUT /snapshot/load` with `snapshot_path: src/vmstate`,
   `mem_backend: {backend_type: "File", backend_path: src/memory}`,
   `network_overrides` that map each `iface_id` to the new host device, and
   `resume_vm: false`.
4. `PUT /mmds` with the metadata of `vm.Spec`, which holds the restore
   markers. The encoding is that of `SetMetadata` today.
5. `PATCH /vm` with `Resumed`.
6. On any error, kill the process, remove the pid file and the socket, and
   return the error. The plan removes the rest.

Memory mode on Firecracker is `FILE` only. The restored process maps
`src/memory`, which stays in the snapshot directory under a reference
(decision 8).

Decision 7. After a load, flintlock puts the MMDS data again, with the
restore markers in it (#1362, #1365). The reasons:

* The restored process starts without `--metadata`, and Firecracker does not
  keep the MMDS data store in a snapshot.
* The client may replace the metadata (SNAP-RST-007), and every restore has
  its own markers (SNAP-RST-008). Both need a fresh put.
* The guest reads the markers to reset its identity (SNAP-CLN-003). The put
  comes before the resume, so the guest never sees the source's data
  without the markers.

The restore also fixes `Metadata()` in `state.go`, which reads
`ConfigPath()` instead of `MetadataPath()`.

### 6.5 Cloud Hypervisor

Cloud Hypervisor is off the critical path. #1376 adds `Pause` and `Resume`
through `vm.pause` and `vm.resume`, and reports `Paused` as a state. #1377
adds `Snapshot` and `Restore`:

* `Snapshot` calls `vm.snapshot` with `destination_url:
  file://<dest>/vmstate`, then sets the files to 0600 and calls `fsync` on
  each (SNAP-PRV-005).
* `Restore` copies `vmstate/config.json` into `<state dir>/restore/` with
  the disk, TAP, vsock and console paths rewritten to the new microVM's
  (SNAP-VOL-007). It starts `cloud-hypervisor --restore` on that directory,
  calls `vm.resume`, and removes `restore/`.
* The memory mode is `COPY` only. A `COPY` restore reads the memory once, so
  it takes no reference.

Cloud Hypervisor has a path override in its saved configuration, so it
needs neither decision 3 nor decision 4. Until #1377 is merged, the provider
does not advertise `snapshot`, and snapshot and restore requests for it are
refused (SNAP-PRV-003).

### 6.6 Capability and eligibility

Decision 10. A provider that can snapshot advertises
`models.SnapshotCapability` (`snapshot`). `CreateSnapshot` and a restore are
refused for a provider without it, through `checkProviderCapabilities`
(SNAP-PRV-003). A microVM can be snapshotted only if all of these hold
(#1373):

| Rule | Reason | Requirement |
| ---- | ------ | ----------- |
| The microVM exists, is not being deleted, and is `created` | A snapshot needs a running source | SNAP-CRT-001, 002 |
| The root is an `overlay` volume | Its writable disk is the only volume with a block-level capture by reflink and a relative path | SNAP-VOL-001, 007 |
| No additional volume, virtiofs mount or `full` volume | Additional volumes have absolute containerd paths (#1379). Virtiofs cannot be snapshotted | SNAP-CRT-004 |
| `allow_guest_agent` is set and `GuestAgentReady` is `true` (#1326) | The guest kernel has finished booting | SNAP-CRT-003 |
| No record of this source is `pending` or `capturing` | One capture per microVM at a time | SNAP-CRT-010 |
| `after` is `RESUME` | `STOP` needs #1330 | SNAP-CRT-006 |

The API checks these before it writes the record. Each refusal is
`FailedPrecondition` (or `NotFound`) with the reason. The first step of the
capture plan checks them again, because the microVM can change in between.

### 6.7 Capture plan

`core/plans/snapshot_capture.go`, with steps in `core/steps/snapshot/`
(#1364). The input is the record, the source microVM and the pause budget.

| Step | Does |
| ---- | ---- |
| 1. `check` | The rules of section 6.6, and the provider state is `running` |
| 2. `pause` | Take the microVM lock. `MicroVMService.Pause`. Start the pause timer |
| 3. `vmm-snapshot` | `MicroVMService.Snapshot(vm, dir)` |
| 4. `disk-capture` | `VolumeService.Capture` of the root |
| 5. `resume` | `MicroVMService.Resume`. Log the pause time. Release the lock |
| 6. `descriptor` | Write `descriptor.json` from the host facts, the spec and the captured volume. Write `spec.json` with the metadata. Both 0600 |

This is the order of SNAP-CRT-005 without quiesce. The disk is captured
while the VMM is paused and its block I/O is drained, so it matches the
memory exactly (SNAP-VOL-001, SNAP-VOL-003). The descriptor is written after
the resume, so it adds nothing to the pause (SNAP-CRT-007). The source's
disk is untouched, so the source can be snapshotted again and deleted
normally (SNAP-VOL-008).

The pause budget covers steps 3 and 4. Its value is `--snapshot-max-pause`.
If it runs out, the plan cancels the step, resumes the source and fails
with `pause budget of <d> exceeded` (SNAP-VOL-005).

### 6.8 Snapshot controller

Decision 11. A separate snapshot controller runs the capture plan. A
per-microVM lock is shared with the microVM reconciler. Two flags limit the
work: `--snapshot-max-pause` (default `10s`) and
`--snapshot-max-concurrent` (default `1`) (#1374).

The controller, `infrastructure/controllers/snapshot_controller.go`, follows
`microvm_controller.go`. It subscribes to the topic `/snapshot`, queues the
snapshot uid, and runs `app.ReconcileSnapshot` with at most
`--snapshot-max-concurrent` workers. A `pending` record moves to
`capturing`, the plan runs, and the record moves to `ready` or `failed`.
Each change publishes `SnapshotUpdated` (SNAP-OPS-001). `ready`, `failed`
and deleted records are a no-op.

`VMLocks` in `core/application` is a keyed mutex by microVM uid. The capture
plan holds the source's lock from before the pause to after the resume.
`ReconcileMicroVM` calls `TryLock`. If the lock is held, it returns and
requeues the microVM after 5 seconds. A `DeleteMicroVM` of the source is
accepted, and its reconcile waits in this way until the capture ends.

The reasons:

* The microVM queue reconciles one microVM per item. A capture is a
  different resource with its own phases. A separate queue limits captures
  without slowing microVM reconciles.
* Without the lock, the reconciler could delete or restart a microVM while
  it is paused for a capture.
* `VMLocks` is in-process. It works because both controllers run in the
  same application instance (`runControllers` in `run.go`). The API server
  never takes it.
* `10s` bounds the stall the guest sees. The disk reflink takes
  milliseconds. Writing a few GB of memory to the page cache takes seconds.
* `1` concurrent capture protects the host. Each capture writes the whole
  guest memory (SNAP-SEC-007).

The `CreateSnapshot` command, the event types and the server are #1373.
`CreateSnapshot` writes a `pending` record and publishes `SnapshotCreated`.
`DeleteSnapshot` of a `pending` or `capturing` snapshot fails with
`FailedPrecondition`. It does not cancel the capture.

### 6.9 Restore

Decision 12, second part: the restore request (#1365). `CreateMicroVM` with
a `snapshot_source`:

1. Loads the record. It is `NotFound` if missing or deleted, and
   `FailedPrecondition` if not `ready`.
2. Resolves the reference to the snapshot uid, once, and stores it in
   `MicroVMSpec.SnapshotSource` (SNAP-RST-014 on one host, amendment S4).
3. Reads `descriptor.json` and `spec.json`, and runs `CheckCompatibility`.
   A mismatch is `FailedPrecondition` with the fields named.
4. Builds the spec from `spec.json` and the overrides (section 5.5).
5. Checks the provider capability and the memory mode. A mode the provider
   does not support is `InvalidArgument`.
6. For a source with a macvtap interface, refuses a second restore
   (section 6.10).
7. Takes the generation from `NextGeneration` and adds the restore markers
   to the metadata.
8. Saves the spec as any other create does.

The restore markers (SNAP-RST-008) are three metadata keys:

| Key | Value |
| --- | ----- |
| `flintlock.snapshot.uid` | The snapshot uid |
| `flintlock.restore.uid` | The uid of the restored microVM |
| `flintlock.restore.generation` | The generation from decision 6 |

The create and update plan uses a restore variant when the spec has a
`SnapshotSource` that is not yet `restored`:

| Step | Does |
| ---- | ---- |
| 1 | The network interfaces of the new microVM (existing steps) |
| 2 | `VolumeService.Restore` of the root, in place of the overlay volume step (SNAP-VOL-006) |
| 3 | `AddRef` when the memory mode is `FILE` |
| 4 | `MicroVMService.Restore`, in place of the create and start steps |
| 5 | Wait for `GuestAgentReady`, with the timeout of #1326 |
| 6 | Set `restored: true` and publish `MicroVMRestored` with the snapshot uid and generation (SNAP-OPS-001) |

The microVM is `created` once the VMM is resumed and `GuestAgentReady` is
`true` (SNAP-RST-012). From then on it is an ordinary microVM, reconciled,
listed and deleted as any other (SNAP-RST-011). `DeleteMicroVM` runs the
existing delete plan, plus `ReleaseRef` for a `FILE` restore.

The base must be on the host. A missing base fails the restore with
`base <digest> of snapshot <uid> is missing`. The milestone does not rebuild
it. The snapshot's `root.base` link keeps it from the sweep.

### 6.10 Clones

The same snapshot restores more than once on one host (SNAP-CLN-001, #1366).
Most of the mechanism is in earlier sections:

| Each restored microVM has its own | From |
| --------------------------------- | ---- |
| uid | section 5.5 |
| state directory and drives | decision 3 |
| vsock socket | decision 4 |
| API socket | decision 4 |
| writable disk | section 5.2 |
| TAP devices | the network overrides and generated TAPs |
| generation | decision 6 |
| reference | decision 8 |

All of them share the base and, in `FILE` mode, the memory file. Spike B
checks that two Firecracker processes can map one memory file at once.

Flintlock does not change the identity inside the guest: hostname, MAC and
IP, machine ID (SNAP-CLN-003). The guest reads the restore markers and acts
on them.

A snapshot of a microVM with a macvtap interface restores at most once
(SNAP-CLN-004). The macvtap device of the guest MAC would be shared. If the
source spec has a macvtap interface and the generation is already 1 or
more, the restore is refused with `FailedPrecondition` before
`NextGeneration` runs.

The user guide (#1375) warns that clones share RNG state, secrets and guest
network identity, and describes VMGenID and the markers as mitigations
(SNAP-CLN-005, SNAP-CLN-006).

### 6.11 Cold boot after a crash

Decision 12, third part. After the first successful restore, a restored
microVM is a plain overlay microVM. If its VMM dies, the reconciler
cold-boots it from its own disk with the normal create plan.
`SnapshotSource` stays in the spec as a record, marked `restored: true`, and
is not used again (#1365).

The reasons:

* The writable disk has moved on since the restore. Loading the snapshot
  memory again would pair old memory with a newer disk. The guest's cached
  filesystem metadata would not match the disk (SNAP-VOL-001).
* A crash of a running microVM is a crash, whatever its origin. A cold boot
  from its own disk is what any other microVM gets.

### 6.12 Daemon settings

| Flag | Default |
| ---- | ------- |
| `--snapshot-max-pause` | `10s` |
| `--snapshot-max-concurrent` | `1` |

The store lives under the existing `--state-dir`. The sockets live under the
existing `--socket-dir`.

## 7. Failure handling

### 7.1 Failures in a request or a plan

| Step | Failure | Handling |
| ---- | ------- | -------- |
| `CreateSnapshot` | A rule of section 6.6 fails | Refuse before any write |
| Capture, `check` | The microVM changed after the API check | Record `failed`. Nothing to resume |
| Capture, steps 3 to 5 | Any error | Resume the source (SNAP-CRT-011). Remove every file but `record.json` (SNAP-OPS-003). Record `failed` |
| Capture, steps 3 and 4 | Pause budget spent | Cancel the step, then as above, with `pause budget of <d> exceeded` |
| Capture, `resume` | The resume fails | Log it and put it in the snapshot error. Release the lock. The reconciler sees `Paused` and leaves the microVM |
| Capture | `FICLONE` not supported | Configuration error, as in `VolumeService.Create` |
| `DeleteSnapshot` | Snapshot `pending` or `capturing` | `FailedPrecondition` |
| `DeleteMicroVM` of a source during a capture | | Accepted. The reconcile waits on the lock |
| Restore, request | Incompatible host, wrong memory mode, second macvtap restore, snapshot not `ready` | Refuse before the spec is saved |
| Restore, plan steps 2 to 5 | Any error | Remove the VMM process, the volumes, the network interfaces and the reference (SNAP-RST-013). Report the microVM failed. The reconciler does not retry it |
| Restore | Base missing | Fail with the error of section 6.9 |
| Firecracker `Restore` | Load or resume fails | Kill the process, remove the pid file and the socket |
| Restored microVM | VMM dies later | Cold boot (section 6.11) |
| Store `Create` | Crash before `record.json` | The startup scan removes the directory |

### 7.2 Startup recovery

Decision 9. Before the snapshot queue starts, `flintlockd` recovers every
record in `pending` or `capturing` (#1374):

1. Set it `failed` with the error `flintlockd restarted during the capture`.
2. Remove its files except `record.json`.
3. If the source microVM is `Paused` at the provider, resume it.

The reasons:

* SNAP-OPS-004 asks for this. Its other branch, a complete package and a
  stop of the source, does not apply: this milestone has no package step
  and no stop.
* A capture cannot continue after a restart. The pause budget is gone, and
  the memory file may be partial.
* A source left paused by the crash must run again (SNAP-CRT-011). The
  reconciler leaves a paused microVM alone, so recovery resumes it.

The store's own scan (section 4.3) runs first and removes directories that
have no record.

### 7.3 Accepted risks

* The base is not hashed again at restore. It is mode 0400 and named by its
  digest, as in section 8 of the overlay boot design. SNAP-VOL-006 and
  SNAP-VOL-011 ask for a verification. On one host the file has never left
  the store (amendment S6).
* A failed snapshot keeps its `record.json` and its name until the client
  deletes it. That keeps the error readable.

## 8. Testing

| Level | Covers | Needs |
| ----- | ------ | ----- |
| Unit | Request bodies of `pkg/firecracker` against a fake socket; the restore command line and working directory; the eligibility rules, each before any write; the capture plan order, clean-up, failed resume and pause budget; the restore overrides, markers, generation and clean-up; the macvtap rule; `CheckCompatibility` per field; the store on a temp dir, with 50 goroutines on `NextGeneration` and a restart; the controller phases, the limit and recovery; `ReconcileMicroVM` requeues while locked | nothing |
| Integration | Capture equals the source disk at the capture, and later source writes do not change it; two restores of one capture get distinct inodes and share the base inode; the sweep keeps a base held only by a snapshot | a reflink filesystem |
| e2e | The success criteria on Firecracker 6.1 (#1367), then on Cloud Hypervisor 6.2 (#1378) | KVM, the reflink state directory of the overlay e2e |

Each criterion is proven from evidence other than the status flintlock
reports:

| Criterion | Evidence |
| --------- | -------- |
| The source kept running | A counter in the guest appends once a second. The gap around the snapshot is under `--snapshot-max-pause` |
| A restore continues the source | A marker file written before the snapshot is there. The counter continues from its value at the snapshot. `/proc/uptime` continues from the source's |
| Markers | The metadata in the guest has the three keys, with generations 1 and 2 |
| Distinct resources | Different inodes of `volumes/root.rw`, TAP names and socket paths. The same inode of `root.base` |
| Isolation of writes | A file written in one microVM is in neither of the others |
| Descriptor | `descriptor.json` has the fields of #1371 and no metadata. `spec.json` is a separate file. All files are 0600 |
| Retained memory | After `DeleteSnapshot`, `ListSnapshots` omits it and `memory` is still there. It goes with the last restored microVM |
| Clean delete | After all deletes, nothing of the microVMs or the snapshot is in the state directory |

On Cloud Hypervisor the memory mode is `COPY`. There `DeleteSnapshot`
removes the whole directory while the restored microVMs run, and a `FILE`
restore is refused.

## 9. Spikes

Both spikes run on Firecracker v1.16.1, the version pinned in
`.github/workflows/hosted_e2e.yml`, before #1362 and #1366 start.

### 9.1 Spike A: relative paths across working directories

Checks decisions 3 and 4, and the request body of section 6.3.

1. Start a microVM from working directory X with relative drive paths
   (`volumes/root.base`, `volumes/root.rw`) and the vsock path
   `sock/guest-agent.vsock`, where `X/sock` is a symlink to a socket
   directory.
2. Pause it and take a snapshot.
3. Make working directory Y with its own `volumes/` and its own `sock` link
   to another socket directory.
4. Start a new Firecracker process with working directory Y, and load the
   snapshot into it.
5. Check that the new process opens Y's drives (`ls -l /proc/<pid>/fd`) and
   binds Y's vsock socket (`ss -xl`).
6. Check whether `PUT /snapshot/create` accepts `sync_snapshot_files` on
   v1.16.1. Record the request body that works.

If A fails, the error goes as a comment on #1362 and #1370, and the
fallback of section 9.3 is chosen here.

### 9.2 Spike B: one memory file in two processes

Checks the memory sharing of section 6.10.

1. Load one snapshot with the `File` memory backend into two Firecracker
   processes at once.
2. Check that both microVMs run.
3. Check that the page cache of the memory file is shared, from
   `/proc/<pid>/smaps` of the mapping or from `fincore` on the file.

If B finds a limit, #1366 carries it out.

### 9.3 Fallback

This applies only if spike A fails. Each Firecracker process runs in its own
mount namespace. In it, the microVM's `volumes/` and `sock` are bind-mounted
to fixed paths, and the snapshot stores those fixed paths. Each restored
process sees its own files at the same paths.

It costs more than decision 3: a mount namespace per process, and bind
mounts to set up before the VMM starts and to tear down after it exits.

### 9.4 Results

The spikes ran on 2026-11-13 on a GitHub-hosted `ubuntu-latest` runner with
KVM (host kernel `7.0.0-1017-azure`), Firecracker v1.16.1 (snapshot format
`v10.0.0`), the guest kernel `ghcr.io/liquidmetal-dev/firecracker-kernel:5.10-20260929-1349`
and a rootfs from `ghcr.io/liquidmetal-dev/ubuntu:24.04`. The working
directories were on XFS with reflink. Run:
<https://github.com/liquidmetal-dev/flintlock/actions/runs/43436483600>.
The script is
[`0204-snapshot-restore-spikes.sh`](0204-snapshot-restore-spikes.sh); the
commands below are the parts that matter.

Each microVM had a working directory `/srv/spike/<name>` with its own
`volumes/root.img` and a link `sock` to `/run/spike/<name>`. Each process
started in its working directory:

```sh
cd /srv/spike/X && firecracker --api-sock /run/spike/X.api --id X
```

and the source was configured through the API with relative paths:

```sh
PUT /drives/root  {"drive_id":"root","path_on_host":"volumes/root.img","is_root_device":true,"is_read_only":false}
PUT /vsock        {"guest_cid":3,"uds_path":"sock/guest-agent.vsock"}
PUT /actions      {"action_type":"InstanceStart"}
```

**Spike A passes.**

- The capture of X: `PATCH /vm {"state":"Paused"}`, then
  `PUT /snapshot/create {"snapshot_type":"Full","snapshot_path":".../vmstate","mem_file_path":".../mem"}`
  returns 204, then `PATCH /vm {"state":"Resumed"}`. `strings vmstate` shows
  that the snapshot keeps the relative paths `volumes/root.img` and
  `sock/guest-agent.vsock`.
- v1.16.1 does not accept `sync_snapshot_files`. The request fails with
  HTTP 400: ``unknown field `sync_snapshot_files`, expected one of
  `snapshot_type`, `snapshot_path`, `mem_file_path` ``. The body above,
  without the field, is the one that works. The provider calls `fsync` on
  both files itself (section 6.3, amendment S7).
- The restore of Y, with X still running: `cd /srv/spike/Y && firecracker
  --api-sock /run/spike/Y.api --id Y`, then `PUT /snapshot/load
  {"snapshot_path":".../vmstate","mem_backend":{"backend_type":"File","backend_path":".../mem"},"resume_vm":true}`
  returns 204, and `GET /` reports `"state":"Running"`.
- `ls -l /proc/<pid>/fd` of Y shows `/srv/spike/Y/volumes/root.img`, not the
  disk of X. `ss -xlp` shows Y listening on `sock/guest-agent.vsock`, and the
  socket file is `/run/spike/Y/guest-agent.vsock`. X still has its own disk
  and socket in `/run/spike/X`, and `GET /` on X reports `Running`.

So relative drive paths and the `sock` link give a restored process its own
drives and its own vsock socket. The fallback (a mount namespace per microVM)
is not needed.

**Spike B passes.**

- A second process Z loaded the same `vmstate` and `mem` with the `File`
  backend while Y ran. Both report `"state":"Running"`. Each opens `mem`
  read-only (`/proc/<pid>/fd`) and has its own disk and socket.
- `/proc/<pid>/smaps` of the 256 MiB mapping of `mem`, the same in both
  processes within a few pages:

  | Field | Y | Z |
  | ----- | - | - |
  | Rss | 43348 kB | 43348 kB |
  | Pss | 22328 kB | 22328 kB |
  | Shared_Clean | 42040 kB | 42040 kB |
  | Private_Dirty | 1304 kB | 1308 kB |

  Pss is about half of Rss, and almost all of Rss is `Shared_Clean`: the two
  processes share the page cache of the file. Each process only holds the
  pages that its guest wrote since the load (`Private_Dirty`, copy on write
  of the `MAP_PRIVATE` mapping). `fincore` is not installed on the runner,
  so the check rests on `smaps`.

Clones of one snapshot on one host (#1366) can therefore share one memory
file, which is what the `refs/<vm-uid>` files of #1372 count.

## 10. Delivery

Every pull request has one concern, a Conventional Commit title and a
sign-off, and every commit compiles with `make test` passing. Pull requests
go against `snapshotrestore`, except #1356 and #1326, which go to `main`.
The branch takes changes by pull request only, with `lint` and `test` green.

The issues of #1358 in the order of the epic:

| Issue | Piece | Blocked by | State on 2026-11-13 |
| ----- | ----- | ---------- | ------------------- |
| #1333 | Cloud Hypervisor client sends the request body (PR #1356, on `main`) | none | merged |
| #1326 | `MicroVMStatus` conditions and `GuestAgentReady` (on `main`) | none | open |
| #1369 | Merge `main` into `snapshotrestore` | #1333, #1326 | open |
| #1359 | This design document and the spikes | none | this pull request |
| #1360 | Firecracker API socket; pause and resume on the port | none | merged, #1386 |
| #1370 | Firecracker relative vsock path | none | merged, #1387 |
| #1361 | API: `SnapshotService` and the snapshot source types | none | merged, #1385 |
| #1371 | Host facts, descriptor, compatibility check | none | merged, #1388 |
| #1363 | Capture and restore of the writable disk | none | open |
| #1372 | Local snapshot store, `SnapshotRepository` | #1361 | open |
| #1362 | Firecracker snapshot and restore on the port | #1360, #1370, #1359 | open |
| #1373 | `SnapshotService` server and commands | #1361, #1372 | open |
| #1364 | Capture plan | #1362, #1363, #1372, #1371, #1369 | open |
| #1374 | Snapshot controller, limits, recovery | #1364, #1373, #1372 | open |
| #1365 | Restore plan | #1361, #1372, #1371, #1362, #1363 | open |
| #1366 | Clones | #1365, #1359 | open |
| #1367 | e2e on Firecracker | #1374, #1365, #1366 | open |
| #1375 | User guide | #1366 | open |
| #1376 | Cloud Hypervisor pause and resume | #1360 | open, PR #1391 |
| #1377 | Cloud Hypervisor snapshot and restore | #1376, #1369 | open |
| #1378 | e2e on Cloud Hypervisor | #1367, #1377 | open |

#1362 waits for spike A of this document, and #1366 for spike B. The
milestone is usable on Firecracker when #1374 and #1366 are merged. #1367
proves it.

The API changes of #1361 are consumed by brigade, battery, controller-pkg
and fl. Their adoption issues are brigade#27, battery#154,
controller-pkg#17 and fl#63, linked from #1385.

## 11. Amendments

| # | Amends | Change |
| - | ------ | ------ |
| S1 | Overlay boot design, A7 | No SQLite store. #1251 is not merged. Snapshot records are files under a flock, rebuilt by a scan (decision 2) |
| S2 | Overlay boot design, section 5.3 | The sweep also counts `snapshots/*/root.base` links as references to a base |
| S3 | SNAP-PKG-011 | The local store is a plain directory in this milestone. The OCI layout comes with push and pull (decision 1) |
| S4 | SNAP-RST-014 | On one host, the reference resolves to the snapshot uid, not to a manifest digest. The uid is in the restore markers |
| S5 | SNAP-META-001 | The descriptor does not yet carry the effective CPU configuration, the interrupt controller version, the quiesce flag, the delta format version, a package format version or a restore policy. Its own `format_version` is 1 |
| S6 | SNAP-RST-004, SNAP-VOL-006, SNAP-VOL-011 | On one host, the CPU configuration is not compared, and the base is not hashed again. The kernel, CPU model and microcode checks stand in for the first. The base never leaves the store for the second |
| S7 | SNAP-PRV-005 | Firecracker v1.16.1 refuses `sync_snapshot_files` (spike A). flintlock calls `fsync` on both files after `PUT /snapshot/create` |
| S8 | SNAP-API-002 | The status has no destination or package digest yet. `local_path` is the only reference |
| S9 | Overlay boot design, section 6.2 | The vsock path of every Firecracker microVM is relative too, through the `sock` link. The API socket replaces `--no-api` |

## 12. References

* Firecracker v1.16.1:
  <https://github.com/firecracker-microvm/firecracker/blob/v1.16.1/docs/snapshotting/snapshot-support.md>,
  <https://github.com/firecracker-microvm/firecracker/blob/v1.16.1/docs/snapshotting/random-for-clones.md>,
  <https://github.com/firecracker-microvm/firecracker/blob/v1.16.1/docs/mmds/mmds-user-guide.md>,
  <https://github.com/firecracker-microvm/firecracker/blob/v1.16.1/docs/vsock.md>,
  <https://github.com/firecracker-microvm/firecracker/blob/v1.16.1/src/firecracker/swagger/firecracker.yaml>,
  <https://github.com/firecracker-microvm/firecracker/blob/v1.16.1/docs/jailer.md>
* Cloud Hypervisor:
  <https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/docs/snapshot_restore.md>
* OCI image layout:
  <https://github.com/opencontainers/image-spec/blob/main/image-layout.md>
* Issues: epic [#1358](https://github.com/liquidmetal-dev/flintlock/issues/1358)
  and the issues of section 10; [#1330](https://github.com/liquidmetal-dev/flintlock/issues/1330),
  [#1379](https://github.com/liquidmetal-dev/flintlock/issues/1379),
  [#1251](https://github.com/liquidmetal-dev/flintlock/pull/1251)
* Flintlock: `core/models/snapshot.go`, `core/models/snapshot_descriptor.go`,
  `core/application/compat.go`, `core/ports/services.go`,
  `infrastructure/microvm/firecracker/state.go`,
  `infrastructure/microvm/firecracker/create.go`,
  `infrastructure/microvm/shared/overlay.go`, `pkg/firecracker/`,
  `api/types/snapshot.proto`,
  `api/services/snapshot/v1alpha1/snapshots.proto`,
  `internal/command/run/run.go`
