# 0204. MicroVM Snapshot & Restore Requirements

* Status: proposed
* Date: 2026-09-22
* Authors: @richardcase
* Issue: [#204](https://github.com/liquidmetal-dev/flintlock/issues/204)

## 1. Conventions

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD",
"SHOULD NOT", "RECOMMENDED", "NOT RECOMMENDED", "MAY", and "OPTIONAL" in this
document are to be interpreted as described in
[BCP 14](https://www.rfc-editor.org/info/bcp14)
([RFC 2119](https://www.rfc-editor.org/rfc/rfc2119),
[RFC 8174](https://www.rfc-editor.org/rfc/rfc8174)) when, and only when, they
appear in all capitals, as shown here.

Each requirement has a unique identifier of the form `SNAP-<AREA>-NNN` so that
it can be referenced from designs, reviews, and tests. Identifiers are stable:
removed requirements are marked as withdrawn rather than renumbered.

Text outside the requirement tables (background, rationale, notes) is
informative.

## 2. Background

Issue [#204](https://github.com/liquidmetal-dev/flintlock/issues/204) asks for
flintlock to expose the snapshot and restore features of the underlying VMMs.
Both [Firecracker](https://github.com/firecracker-microvm/firecracker/tree/main/docs/snapshotting)
and [Cloud Hypervisor](https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/docs/snapshot_restore.md)
can pause a running microVM, write its memory and device state to files, and
later start a new VMM process from those files. Neither VMM packages,
transports, secures, or manages those files; that is left to the integrator.

Today flintlock has no support for any part of this:

* The `MicroVM` gRPC service has no pause, resume, or snapshot RPCs, and the
  `MicroVMService` provider port (`core/ports/services.go`) has no equivalent
  methods.
* Firecracker is started with `--no-api`
  (`infrastructure/microvm/firecracker/create.go`), so there is no control
  channel to a running Firecracker process.
* The Cloud Hypervisor client (`pkg/cloudhypervisor/client.go`) has `Pause` and
  `Resume` calls that are unused, and no snapshot or restore calls.
* The `ImageService` port can pull images but cannot push them, and has no
  registry credential support.
* The guest agent ([liquidmetal-dev/guest-agent](https://github.com/liquidmetal-dev/guest-agent),
  enabled with `allow_guest_agent`) offers exec and ssh-proxy only.

### 2.1 Goals

* Take a snapshot of a running microVM and package it as a portable artifact
  that can be stored and restored later.
* **Cross-host restore**: restore a snapshot on a different, compatible
  flintlock host (this also covers the backup use case).
* **Clone / fan-out**: create several microVMs from one snapshot.
* Record enough metadata with a snapshot to decide, before restoring, whether a
  host can restore it.
* Optionally restrict where and by whom a snapshot can be restored, using a
  restore policy.

### 2.2 Non-goals

* Live migration.
* Diff / incremental snapshots (Firecracker marks these as developer preview;
  Cloud Hypervisor does not support them).
* Restoring a snapshot using a different VMM from the one that created it.
* Scheduling snapshots inside flintlock. Periodic snapshots are driven by
  callers (for example CAPMVM or an external scheduler).
* Per-clone network namespace isolation. This is tracked separately.
* Encryption of snapshot packages. This will be addressed in a follow-up
  change (see [section 5.12](#512-future-work-encryption-informative)).

## 3. Terminology

| Term | Meaning |
| ---- | ------- |
| Snapshot | The point-in-time state of a microVM: VMM device state, guest memory, and the block state of every mounted volume. Also the flintlock API resource that tracks producing it. |
| Snapshot package | The OCI artifact that holds a snapshot and its metadata. |
| Source VM | The microVM a snapshot was taken from. |
| Restored VM | A microVM created from a snapshot package. |
| Clone | One of several restored VMs created from the same snapshot package. |
| Compatibility descriptor | Plaintext metadata in the snapshot package that describes the VMM, host, and images the snapshot depends on. |
| Quiesce | Flushing and freezing the guest's writable filesystems so that applications see a clean point in time. Optional; see section 5.3. |
| Thaw | Reversing a quiesce. |
| Writable volume | A root or additional volume that is not `is_read_only`. |
| Mounted volume | Any root or additional volume, writable or read-only, presented to the guest as a block device. The kernel and initrd are not volumes: they are loaded into memory at boot and not read afterwards. |
| Base block image | The exact block content a volume was created from: the committed image snapshot's device or file, as built on the source host. Identified by the digest of its content, not by the OCI image digest alone, and located by a qualified reference (SNAP-PKG-006). |
| Volume delta | The set of blocks of a volume that differ from its base block image, addressed by offset within the volume (not by physical location in a pool or file). |

## 4. VMM capability summary (informative)

| | Firecracker | Cloud Hypervisor |
| - | ----------- | ---------------- |
| Pause / resume | `PATCH /vm {"state": "Paused" \| "Resumed"}` | `vm.pause` / `vm.resume` |
| Create snapshot | `PUT /snapshot/create` (`snapshot_path`, `mem_file_path`, `snapshot_type`, `sync_snapshot_files`); VM has to be paused | `vm.snapshot` to a `file://` destination directory; VM has to be paused |
| Restore | `PUT /snapshot/load` on a fresh process, before boot | `--restore source_url=...` at launch, or `vm.restore` on an empty VMM |
| Files produced | VM state file (with CRC) and guest memory file | `config.json`, `state.json`, `memory-ranges` (mode 0600) |
| Disk formats | Raw file only | Raw, qcow2 (backing files opt-in with `backing_files=on`), vhd, vhdx, vmdk |
| Disk state at snapshot | Pause drains in-flight I/O; create fsyncs the backing files; the user must back them up externally | VM has to be paused; disks are to be copied while paused |
| Disk path override at restore | None: disks must be at the same relative paths (issue [#4014](https://github.com/firecracker-microvm/firecracker/issues/4014)), so the stored path has to resolve per process for same-host clones | None in the API; `config.json` is documented as editable between snapshot and restore |
| Swap a mounted disk on a live VM | `PATCH /drives/{id}` only for a device the guest has not mounted, and not atomic | Only `vm.add-disk` / `vm.remove-device` |
| Disk snapshot API | None | None |
| Memory backends | `File` (lazy, `MAP_PRIVATE`) or `Uffd` (external page-fault handler) | `copy` (default, eager), `ondemand` (userfaultfd), `copyonwrite` (shared page cache) |
| Diff snapshots | Developer preview | Not supported |
| Version rules | Each binary supports exactly one snapshot format version; snapshots are compatible only if the effective guest-visible CPU configuration is invariant (a matching template name is not enough); Intel to AMD is unsupported; host kernel changes are "unstable" | No documented guarantees |
| Not captured | Disk contents, MMDS data store, network connections, logger/metrics config; vsock is reset | Disk contents |
| Host resource remapping | `network_overrides`, `vsock_override` (no drive override) | `net_fds`, or editing `config.json` |
| vhost-user block | Developer preview; snapshots are not supported for VMs with vhost-user devices | Supported (`vhost_user_block` is raw only) |
| Uniqueness | Always-on VMGenID; secure only if each snapshot is resumed once | Not documented |

Both VMMs expect disks, TAP devices, and sockets to exist at the same paths at
restore time unless remapped. Neither VMM can remap a drive through its restore
API, and neither can change a mounted disk under a running guest. The
supporting research is in
[0204-root-volume-options.md](0204-root-volume-options.md).

## 5. Requirements

### 5.1 Provider (SNAP-PRV)

| ID | Requirement |
| -- | ----------- |
| SNAP-PRV-001 | The Firecracker provider MUST start Firecracker with an API socket for every microVM, replacing the current `--no-api` start. |
| SNAP-PRV-002 | The provider port MUST support pausing, resuming, creating a snapshot of, and restoring a microVM. |
| SNAP-PRV-003 | A provider that supports snapshots MUST advertise a `snapshot` capability, and flintlock MUST reject snapshot and restore requests for providers without it, using the existing capability check (`checkProviderCapabilities` in `core/application/commands.go`). |
| SNAP-PRV-004 | The Firecracker and Cloud Hypervisor providers MUST both implement the snapshot capability. |
| SNAP-PRV-005 | Snapshot files MUST be synced to stable storage before the snapshot is reported as captured (for Firecracker, `sync_snapshot_files` MUST be `true`). |
| SNAP-PRV-006 | Providers MUST report the paused state of a microVM rather than treating it as an unsupported state. |

### 5.2 Snapshot creation (SNAP-CRT)

| ID | Requirement |
| -- | ----------- |
| SNAP-CRT-001 | A client MUST be able to request a snapshot of a microVM that is running. |
| SNAP-CRT-002 | Flintlock MUST reject a snapshot request for a microVM that is not running. |
| SNAP-CRT-003 | Flintlock SHOULD reject, or warn about, a snapshot request made before the guest kernel has finished booting (Firecracker documents that such snapshots can crash the guest on resume). |
| SNAP-CRT-004 | Flintlock MUST reject a snapshot request before pausing the microVM if the microVM has any device that cannot be snapshotted. Virtiofs volumes MUST be treated as unsupported. |
| SNAP-CRT-005 | Snapshot creation MUST follow this order: optional quiesce (SNAP-QSC), pause, VMM snapshot, point-in-time block capture of every mounted volume (SNAP-VOL). Then, if the client chose resume, flintlock MUST resume (and thaw) the source VM and package afterwards. If the client chose stop, the source VM MUST remain paused while the package is built in the local store, and MUST be stopped once the package is complete and verified (the stop point). |
| SNAP-CRT-006 | The client MUST be able to choose whether the source VM is resumed or stopped after the snapshot. The default MUST be to resume. The stop takes effect at the stop point defined in SNAP-CRT-005, after the package is complete and verified in the local store (SNAP-PKG-011, SNAP-PKG-012), and not before (SNAP-CRT-011). |
| SNAP-CRT-007 | When the source VM is to be resumed, compression and packaging MUST take place after the resume, so that they do not add to the pause time. When it is to be stopped, they take place while it is still paused (SNAP-CRT-005), and that time MUST NOT count against the maximum pause duration (SNAP-VOL-005). Pushing MUST take place after the resume or the stop in both cases. |
| SNAP-CRT-008 | Snapshot creation MUST be asynchronous: the request MUST return a snapshot identifier, and the client MUST be able to observe progress and the final result. |
| SNAP-CRT-009 | Flintlock MUST produce only full snapshots. |
| SNAP-CRT-010 | Flintlock MUST NOT allow more than one snapshot of the same microVM to be in progress at a time. |
| SNAP-CRT-011 | If snapshot creation fails at any point after the pause and before the stop point (SNAP-CRT-005), flintlock MUST return the source VM to its prior running state (resumed, and thawed if it was quiesced), regardless of whether the client asked for it to be stopped; the client MAY stop it explicitly afterwards. If the push fails after the stop point, the snapshot MUST remain available from the local store with the failure reported, the push MAY be retried, and the source VM MUST NOT be restarted. |

### 5.3 Quiesce (SNAP-QSC)

Volumes are captured at block level while the VMM is paused and its block I/O
is drained (SNAP-VOL). The captured blocks are therefore exactly what the guest
kernel's memory expects, in the same way that a running VM is consistent with
its own disk, so quiescing is not needed for filesystem consistency. It remains
useful for *application* consistency (for example flushing a database before
the snapshot) and is offered as an option. A quiesced source VM that is kept
paused through packaging because the client chose stop (SNAP-CRT-005) stays
frozen and is never thawed, since it is stopped rather than resumed.

| ID | Requirement |
| -- | ----------- |
| SNAP-QSC-001 | Flintlock MUST NOT require a quiesce for a snapshot. The client MAY request one. (Revised: quiesce was mandatory for writable volumes when volume capture was file-level.) |
| SNAP-QSC-002 | When requested, quiescing MUST be performed through the guest agent. A quiesce request for a microVM that was not created with `allow_guest_agent` MUST be rejected. |
| SNAP-QSC-003 | The guest agent MUST provide dedicated freeze and thaw operations. Freeze MUST find the guest's writable filesystems itself, sync them, and freeze them (for example with `FIFREEZE`). This is a dependency on the guest-agent project. |
| SNAP-QSC-004 | The freeze operation MUST take a timeout after which the guest agent thaws the filesystems on its own, so that a guest is never left frozen if flintlock fails. |
| SNAP-QSC-005 | If a requested quiesce fails, the snapshot MUST fail, unless the client set an explicit force flag. Whether the snapshot was quiesced MUST be recorded in its metadata (SNAP-META-001). |
| SNAP-QSC-006 | If the source VM was quiesced, flintlock MUST thaw it after the VMM snapshot and volume capture, whether they succeeded or failed, if the source VM is resumed. |
| SNAP-QSC-007 | A VM restored from a quiesced snapshot resumes with its filesystems frozen. Flintlock MUST thaw such a VM through the guest agent after resuming it, and MUST report it as failed if thawing does not succeed within a configurable timeout. VMs restored from a non-quiesced snapshot need no thaw. |

### 5.4 Volumes (SNAP-VOL)

| ID | Requirement |
| -- | ----------- |
| SNAP-VOL-001 | On restore, every mounted volume MUST be presented to the guest with contents byte-identical to the source volume at the instant of the VMM snapshot. Volume capture MUST therefore be block-level. File-level diffs MUST NOT be used to reconstruct a volume for a memory restore, because the restored guest kernel's cached filesystem metadata refers to block addresses on the source disk. (Revised: the first draft required a file-level diff applied to a fresh unpack of the image, which cannot satisfy this.) |
| SNAP-VOL-002 | The kernel and initrd MUST NOT be copied into the snapshot package. They MUST be referenced by OCI image digest (SNAP-PKG-006). (Revised: read-only volumes are no longer exempt; see SNAP-VOL-010.) |
| SNAP-VOL-003 | The source VM MUST remain paused until every mounted volume has been captured at the same point in time as the VMM snapshot. |
| SNAP-VOL-004 | Flintlock SHOULD use the capture mechanism with the shortest pause that the VMM and storage backend support (see the capture options below). When the mechanism's pause grows with the size of the volume, flintlock SHOULD log a warning. |
| SNAP-VOL-005 | Flintlock SHOULD support a configurable maximum pause duration covering the VMM snapshot and volume capture. If the capture would exceed it, flintlock SHOULD abort the snapshot and resume the source VM. Packaging time for a source VM that will be stopped (SNAP-CRT-007) is not subject to this limit. |
| SNAP-VOL-006 | On restore, flintlock MUST recreate each mounted volume, before starting the VMM, by obtaining its base block image (SNAP-VOL-011), verifying it, and applying the volume delta (or writing the full volume image when the package carries one). |
| SNAP-VOL-007 | Restored volumes MUST be presented to the VMM with the device ordering and drive identifiers the snapshot expects. Every restore MUST create the restored VM's volumes at host paths unique to that VM, never reusing the source VM's or another clone's paths. For Cloud Hypervisor, flintlock MUST rewrite the disk paths in the saved configuration to those paths before restore. For Firecracker, which has no drive override at load time and reopens each block device at the path stored in the snapshot, the stored path MUST resolve, in the restored Firecracker process, to that VM's own volume. The mechanism (for example a relative path with a per-VM working directory, a per-VM mount namespace, or the jailer's chroot) is left to the design. |
| SNAP-VOL-008 | After the capture, the source VM's volumes MUST remain usable by the source VM and MUST remain correctly tracked by containerd, so that the source VM can be snapshotted again and deleted normally. |
| SNAP-VOL-009 | For each mounted volume, the snapshot package MUST carry either a volume delta together with the digest of the base block image it applies to, or a full block image of the volume. Deltas MUST be expressed in offsets within the volume, never in physical pool or file offsets, and MUST record the block size used. |
| SNAP-VOL-010 | Read-only mounted volumes MUST be treated like writable volumes for restore purposes: the package MUST reference their base block image by digest so that the target presents identical content. They MUST NOT be reconstructed from a fresh unpack of the OCI image. |
| SNAP-VOL-011 | Flintlock MUST be able to obtain a base block image on the target host by its digest, either by pulling it as a content-addressed artifact (SNAP-PKG-006, SNAP-PKG-007) or by building it deterministically from the OCI image, and MUST verify the digest before use. A restore MUST fail if the base block image cannot be obtained or does not match. |
| SNAP-VOL-012 | Captured volume state and restored volumes that flintlock creates outside containerd's metadata MUST be tracked by flintlock and removed when the snapshot record or restored VM is deleted (SNAP-API-005, SNAP-RST-011). |

#### Capture options (informative)

With the devmapper snapshotter, a mounted volume (including the root volume)
is an *active* containerd snapshot whose parent is the committed image
snapshot. containerd has no operation that takes a point-in-time copy of an
active snapshot. This is true of v1.7 (which flintlock uses) and remains true
in v2.3 and on `main` as of September 2026: `Prepare` and `View` require a
committed parent, and `Commit` deactivates the device, which a running VM is
still using. dm-thin itself can snapshot an active device instantly, but
containerd does not expose this. A proposal to add snapshot and restore
operations to snapshotters
([containerd PR #13111](https://github.com/containerd/containerd/pull/13111))
was closed without merging.

Two mechanisms considered in the first draft have been ruled out:

* *Commit and swap* with Firecracker's `PATCH /drives/{id}`: Firecracker only
  permits the update for a device the guest has not mounted, so a mounted root
  cannot be swapped.
* *Mount the frozen filesystem read-only on the host and diff it with
  containerd's differ*: this yields a file-level diff, which cannot be used for
  a memory restore (SNAP-VOL-001).

The remaining mechanisms depend on the storage backend, which is not yet
decided. They are compared in detail in
[0204-root-volume-options.md](0204-root-volume-options.md).

| Backend and mechanism | Pause | Delta against base | VMMs | Tracked by containerd | Notes |
| --------------------- | ----- | ------------------ | ---- | --------------------- | ----- |
| devmapper (current): `dmsetup suspend` the VM's thin device, `create_snap`, resume, all while the VMM is paused | Milliseconds | `thin_delta` against the image device, using a reserved metadata snapshot; offsets are volume-virtual | Both | No: device IDs must be reserved so they cannot collide with containerd's allocator, and the devices are invisible to containerd GC | The kernel requires the origin to be suspended for `create_snap`. |
| containerd `blockfile` snapshotter on XFS (`reflink=1`) or Btrfs: `FICLONE` the VM's raw image file while paused | Milliseconds | None native; compare against a reflinked base, or ship the full sparse file | Both | No | Small provisioning change; the snapshotter is young and does not preserve sparseness on copy. |
| Read-only base block image (for example EROFS) plus a separate writable raw disk, overlay assembled by the guest: reflink or copy the writable disk while paused | Milliseconds with reflink | The writable disk is the delta; the base is deterministic or content-addressed | Both | Base only | Changes the guest image contract (needs an overlay-capable init). |
| qcow2 overlay over a digest-named raw base: copy the overlay while paused | Milliseconds with reflink | The overlay is the delta | Cloud Hypervisor only | No | Firecracker has no qcow2 support, and vhost-user disables its snapshots. |
| Stop the source VM, then capture the whole device or file | None (the VM is stopped) | Any of the above | Both | Depends on backend | Only when the client asks for the source VM to be stopped (SNAP-CRT-006). |
| Extend containerd's snapshotters to allow a point-in-time snapshot of an active snapshot | Milliseconds | Backend-specific | Both | Yes | Needs an upstream change; see open questions. |

In every case the base block image on the target must be byte-identical to the
one on the source (SNAP-VOL-011). The mechanism for that, shipping the base as
an artifact or building it deterministically, is also undecided.

### 5.5 Packaging (SNAP-PKG)

| ID | Requirement |
| -- | ----------- |
| SNAP-PKG-001 | A snapshot package MUST be an OCI artifact as defined by the [OCI image specification](https://github.com/opencontainers/image-spec), with a flintlock-specific `artifactType` and media types. |
| SNAP-PKG-002 | The package's config blob MUST hold the snapshot metadata (SNAP-META). |
| SNAP-PKG-003 | The VMM state files, guest memory, and each volume delta or full volume image MUST each be stored in separate layers, identified by media type and annotations. Base block images, when shipped, MUST be separate artifacts referenced by digest rather than layers of the snapshot package. |
| SNAP-PKG-004 | Guest memory layers MUST be compressed with zstd. |
| SNAP-PKG-005 | Guest memory SHOULD be split into multiple layers of bounded size, so that push and pull can run in parallel and resume after interruption. |
| SNAP-PKG-006 | The package MUST record, for every OCI image the snapshot depends on (kernel, initrd, and the images the volumes were created from) and for every base block image the volume deltas apply to, both its digest and a fully qualified reference (registry, repository, and digest) from which it can be fetched. A digest alone identifies content but does not locate it: the OCI distribution API fetches manifests and blobs by repository name and digest. The digest is the identity; flintlock MAY be configured with alternative repositories or mirrors to try, and MUST verify whatever it fetches against the recorded digest (SNAP-PKG-012). References by tag alone MUST NOT be used. |
| SNAP-PKG-007 | The package SHOULD also link to the manifests of the images and base block image artifacts it depends on, through an OCI index or the subject/referrers mechanism, so that tools copying the snapshot between registries copy those too and registry garbage collection does not remove them. |
| SNAP-PKG-008 | The package MUST carry a flintlock snapshot package format version. Flintlock MUST refuse to restore a package whose major format version it does not support. |
| SNAP-PKG-009 | Flintlock MUST support pushing a snapshot package to an OCI registry. |
| SNAP-PKG-010 | Registry credentials MUST be configured on the flintlockd host, per registry (for example with a docker `config.json`-style file). The same credentials MUST be used to pull snapshot packages and their images on restore. Credentials MUST NOT be passed in API requests. |
| SNAP-PKG-011 | Flintlock MUST support storing a snapshot package locally, as an [OCI image layout](https://github.com/opencontainers/image-spec/blob/main/image-layout.md) directory under the flintlock state directory, so it can be copied with standard tools such as `oras`. The layout MUST either contain the package's dependencies (base block images, and any image not otherwise obtainable) in its index or carry their qualified references (SNAP-PKG-006), so that a copied layout is restorable on a host that holds none of them. |
| SNAP-PKG-012 | Flintlock MUST verify the digest of every blob it reads from a snapshot package. |

### 5.6 Metadata (SNAP-META)

| ID | Requirement |
| -- | ----------- |
| SNAP-META-001 | The package MUST contain a plaintext compatibility descriptor with at least: VMM type; exact VMM version; VMM snapshot format version (Firecracker); flintlock version; package format version; host CPU architecture; CPU vendor and model; the CPU template used, if any; for Firecracker, the effective guest-visible CPU configuration captured on the source host (CPUID leaves and MSRs on x86_64, system registers on arm64), or a digest of it; host kernel version; interrupt controller version (arm64 GIC); vCPU count and memory size; digests of the images the snapshot depends on; for each mounted volume, its drive identifier and order, size, block size, filesystem type, base block image digest, and delta format version; whether the snapshot was quiesced; creation time; source VM UID, name, and namespace; client-supplied labels; and the restore policy (SNAP-SEC-002), if any. |
| SNAP-META-002 | Flintlock MUST be able to read and evaluate the compatibility descriptor without reading the guest memory, VMM state, or volume layers. |
| SNAP-META-003 | The package MUST include the full microVM spec of the source VM, including the `metadata` map, separately from the compatibility descriptor. Because the `metadata` map often holds secrets such as cloud-init user data, and is stored unencrypted in this version, the spec MUST be treated as sensitive (SNAP-SEC-001). |
| SNAP-META-004 | The compatibility descriptor MUST NOT contain the source VM's `metadata` map or any other guest-supplied secret. |
| SNAP-META-005 | Clients MUST be able to attach labels to a snapshot at creation time; these MUST be stored in the compatibility descriptor. |

### 5.7 Restore (SNAP-RST)

| ID | Requirement |
| -- | ----------- |
| SNAP-RST-001 | A client MUST be able to create a microVM from a snapshot package in a registry or in the local store, through the same creation path as other microVMs (SNAP-API-003). |
| SNAP-RST-002 | The client MAY override the restored VM's name, namespace, UID, labels, metadata, and host-side network devices. |
| SNAP-RST-003 | Flintlock MUST reject a restore request that changes vCPU count, memory size, CPU configuration, or the set and order of devices. |
| SNAP-RST-004 | Before starting the VMM, flintlock MUST check the compatibility descriptor against the host and MUST reject the restore, with an error naming the mismatch, if any of these differ: VMM type, CPU architecture, interrupt controller version; for Firecracker, the snapshot format version, the CPU vendor, and the effective guest-visible CPU configuration recorded in the descriptor, which MUST match the target host's exactly; for Cloud Hypervisor, the exact VMM version. A matching CPU template name is not sufficient for the Firecracker check, because the configuration a template produces depends on the host's BIOS, CPU, kernel, and Firecracker version. Firecracker documents that snapshots are compatible only when the guest-visible CPU features are invariant and that Intel to AMD restores are unsupported, so these checks MUST NOT be skippable. |
| SNAP-RST-005 | Flintlock SHOULD also reject the restore if the host kernel version differs (Firecracker documents restores across host kernels as unstable), if the CPU model name differs while the effective CPU configuration matches, or, for Cloud Hypervisor, if the CPU vendor, model, or feature flags differ. The client MAY set a force flag to skip the checks in this requirement, but not those in SNAP-RST-004. |
| SNAP-RST-006 | Flintlock MUST check the restore policy (SNAP-SEC-002) and signature (SNAP-SEC-004) before pulling the guest memory, VMM state, or volume layers. |
| SNAP-RST-007 | By default, flintlock MUST give the restored VM the source VM's metadata. The client MAY supply replacement metadata. |
| SNAP-RST-008 | Flintlock MUST add restore markers to the restored VM's metadata: the snapshot identifier, the restored VM's UID, and a restore generation counter that is unique for each restore of the same snapshot. |
| SNAP-RST-009 | The client MAY choose the memory restore mode. The default MUST be `copy` for Cloud Hypervisor and `File` for Firecracker. Support for Firecracker `Uffd` MAY be added later. |
| SNAP-RST-010 | When a lazy memory mode is used (Firecracker `File` or `Uffd`, Cloud Hypervisor `ondemand` or `copyonwrite`), flintlock MUST keep the memory file in place and unchanged, protected by a containerd lease or equivalent, until every VM using it has been deleted. The same protection MUST cover any other package content a restored VM reads after it has started. |
| SNAP-RST-011 | A restored VM MUST be managed like any other microVM: it MUST be reconciled, reported through `GetMicroVM` and `ListMicroVMs`, and deleted through `DeleteMicroVM`, including clean-up of its restored volumes and memory files. |
| SNAP-RST-012 | A restored VM MUST be resumed after restore, and MUST be reported as created only after it has been resumed and, if the snapshot was quiesced, thawed (SNAP-QSC-007). |
| SNAP-RST-013 | If a restore fails, flintlock MUST remove any partial state it created (VMM process, volumes, network devices, local copies of the package) and report the microVM as failed. |
| SNAP-RST-014 | Flintlock MUST resolve the package reference to a manifest digest once, before any compatibility, policy, or signature check, MUST fetch the manifest by that digest, and MUST record it in the restore markers (SNAP-RST-008). The config and each layer are then fetched by the digests in that manifest's descriptors and verified (SNAP-PKG-012). Each dependency (images and base block images) is fetched by its own digest as recorded in the package (SNAP-PKG-006), never by tag. The client MAY supply a digest reference directly; a tag reference is resolved at this step and MUST NOT be re-read during the restore. |

### 5.8 Clones (SNAP-CLN)

Restoring one snapshot more than once duplicates everything held in guest
memory: identifiers, tokens, keys, RNG state, and the guest's MAC and IP
addresses. Firecracker documents that the only secure pattern is to resume each
snapshot exactly once.

| ID | Requirement |
| -- | ----------- |
| SNAP-CLN-001 | Flintlock MUST allow the same snapshot package to be restored more than once, on the same or different hosts. |
| SNAP-CLN-002 | Each restored VM MUST have a unique UID, unique host-side network devices, a unique vsock path, and unique host paths for its volumes (SNAP-VOL-007). |
| SNAP-CLN-003 | Flintlock MUST NOT attempt to change guest-side identity (hostname, MAC and IP inside the guest, machine ID). The guest is responsible for this, using the restore markers (SNAP-RST-008) and any replacement metadata. |
| SNAP-CLN-004 | Snapshots of microVMs with macvtap interfaces MUST NOT be restored more than once in this version. |
| SNAP-CLN-005 | The user documentation MUST warn that clones share RNG state, secrets, and guest network identity, MUST describe the mitigations, and MUST state the per-architecture kernel requirement for VMGenID (SNAP-CLN-006), including that arm64 guests on Firecracker need the DeviceTree binding backported to a 6.1 kernel. |
| SNAP-CLN-006 | Guests that will be cloned SHOULD use a kernel with VMGenID support so that the kernel RNG is reseeded on restore. On ACPI systems (x86_64) that is Linux 5.18 or later. On arm64, which uses DeviceTree, it is Linux 6.10 or later; because the newest guest kernel Firecracker supports is 6.1, arm64 guests on Firecracker need a 6.1 kernel with the VMGenID DeviceTree binding backported from 6.10, as Firecracker's own CI does. Without it, clones are not reseeded. |

### 5.9 Security and access control (SNAP-SEC)

Guest memory contains everything the guest holds in RAM, including keys and
credentials, and the stored microVM spec contains the guest's metadata, which
often includes secrets. A snapshot package is therefore as sensitive as the
running VM. Snapshot packages are not encrypted in this version (see
[section 5.12](#512-future-work-encryption-informative)), so anyone who can
read a package can read those secrets and restore the VM. Access to packages
has to be controlled with registry permissions and local file permissions. The
restore policy only binds well-behaved flintlock hosts.

| ID | Requirement |
| -- | ----------- |
| SNAP-SEC-001 | Flintlock MUST treat snapshot files and packages as sensitive. Local snapshot files MUST be created with mode 0600 in directories owned by flintlockd. |
| SNAP-SEC-002 | The client MAY attach a restore policy to a snapshot, listing allowed target hosts (by host identifier or host labels) and allowed client identities. When a package has a policy, flintlockd MUST reject restores that the policy does not allow. |
| SNAP-SEC-003 | Client identities in a restore policy MUST be matched against the mTLS client certificate subject. A policy that lists client identities MUST be rejected by a host that does not validate client certificates (`--tls-client-validate`). |
| SNAP-SEC-004 | Flintlock SHOULD support signing snapshot packages (for example with cosign or notation). When a package carries a restore policy, flintlockd MUST verify its signature against public keys configured on the host and MUST reject the restore if verification fails. Because an unsigned package can simply omit a policy, this check on its own only protects a signed package against tampering; it does not stop a registry writer from publishing a stripped, unsigned copy. Enforcing policies against that requires SNAP-SEC-005. |
| SNAP-SEC-005 | A flintlockd host MAY be configured to require a valid signature for every restore. A deployment that relies on restore policies MUST enable this on every host that can restore, since it is the only way to reject a package whose policy has been removed. |
| SNAP-SEC-006 | The user documentation MUST state that snapshot packages are not encrypted, that they contain the guest's secrets (in guest memory and in the stored spec metadata), that the restore policy is not a security boundary on its own and is enforceable against stripping only on hosts that require signatures (SNAP-SEC-005), that tag references are resolved to a digest at the start of a restore (SNAP-RST-014), and that access to packages has to be restricted with registry and storage permissions. |
| SNAP-SEC-007 | Flintlock SHOULD support configurable limits on local snapshot storage and on the number of concurrent snapshot operations, to prevent snapshots exhausting host resources. |
| SNAP-SEC-008 | The package format SHOULD allow encryption to be added in a later version without a new major package format version (SNAP-PKG-008), for example by staying compatible with the [ocicrypt](https://github.com/containers/ocicrypt) encrypted layer media types. |

### 5.10 API (SNAP-API)

| ID | Requirement |
| -- | ----------- |
| SNAP-API-001 | Snapshots MUST be exposed through a new `SnapshotService` in `snapshot.services.api.v1alpha1`, with operations to create a snapshot, get a snapshot, list snapshots, stream a list of snapshots, and delete a snapshot. Each operation MUST have a grpc-gateway REST mapping, following the pattern in `api/services/microvm/v1alpha1/microvms.proto`. |
| SNAP-API-002 | A snapshot MUST be a first-class resource with an identifier, the source VM, the destination (registry reference or local store), labels, and a status. The status MUST include a phase (pending, quiescing, capturing, packaging, pushing, ready, failed), an error message on failure, and on success the package's reference and digest. A package that is complete in the local store but not yet pushed MUST keep its local reference in the status. |
| SNAP-API-003 | `CreateMicroVM` MUST accept either a microVM spec (as today) or a snapshot source. A snapshot source MUST contain the package reference (a tag or a digest; see SNAP-RST-014), the overrides allowed by SNAP-RST-002, the memory restore mode, and the force flag (SNAP-RST-005). |
| SNAP-API-004 | All API changes MUST be backwards compatible with existing `v1alpha1` clients. |
| SNAP-API-005 | Deleting a snapshot MUST remove its record and any local package content that no restored VM references. Content still in use by a restored VM (the memory file in a lazy memory mode, SNAP-RST-010) MUST be retained under the lease or reference count that protects it and removed only when the last such VM is deleted. Flintlock MUST NOT delete snapshot packages from a remote registry. |
| SNAP-API-006 | Deleting a snapshot MUST NOT affect restored VMs that are using its content; retained content is released only when those VMs have been deleted (SNAP-API-005, SNAP-RST-010). |
| SNAP-API-007 | `ServerInfo` SHOULD report the VMM types and versions available on the host, the host's compatibility descriptor fields, and, for Firecracker, a digest of the host's effective guest-visible CPU configuration for each available CPU template, so clients can pick a compatible host before restoring. |

### 5.11 Operations (SNAP-OPS)

| ID | Requirement |
| -- | ----------- |
| SNAP-OPS-001 | Flintlock MUST publish events for snapshot phase changes and for restores. |
| SNAP-OPS-002 | Flintlock SHOULD expose metrics for pause duration, snapshot size, packaging time, push time, and restore time. |
| SNAP-OPS-003 | If a snapshot operation fails, flintlock MUST remove partial local artifacts. |
| SNAP-OPS-004 | If flintlockd restarts while a snapshot is in progress and the package is complete and verified in the local store, it MUST stop the source VM if the client chose stop, report the snapshot with its local reference, and MAY retry the push. Otherwise it MUST mark the snapshot as failed, remove partial artifacts, and thaw and resume the source VM if it is still frozen or paused, regardless of the client's stop choice (SNAP-CRT-011). |
| SNAP-OPS-005 | Snapshot records MUST persist across flintlockd restarts. |

### 5.12 Future work: encryption (informative)

Encrypting snapshot packages is out of scope for this version and will be
addressed in a follow-up change. The expected direction is:

* Each snapshot can be encrypted or not, and encryption covers the guest memory, VMM state,
  writable volume diffs, and stored microVM spec.
* It is recipient-based and compatible with
  [ocicrypt](https://github.com/containers/ocicrypt): each layer is encrypted
  with a random data key, which is wrapped for one or more recipients' public
  keys. Only holders of a matching private key can restore the snapshot.
* The private key is supplied only in the restore request and is never
  persisted or logged.
* The compatibility descriptor gains an encryption descriptor, and stays
  readable without decrypting the package (SNAP-META-002).

SNAP-SEC-008 keeps the package format open to this change.

## 6. Known limitations and risks (informative)

* **Network connections are lost.** Packets in flight at snapshot time are
  dropped and TCP connections are not guaranteed to survive. Firecracker resets
  vsock, closing guest agent connections.
* **Firecracker MMDS data is not restored.** Flintlock re-supplies metadata on
  restore (SNAP-RST-007).
* **The guest clock jumps.** The guest resumes with the wall-clock time of the
  snapshot and has to correct it. Firecracker's `clock_realtime` load option MAY
  be used later on x86_64 hosts.
* **Volume capture happens outside containerd.** No containerd snapshotter can
  take a point-in-time copy of an active snapshot, so flintlock has to capture
  block state itself (dm-thin messages, reflinks, or file copies) and track the
  results outside containerd's metadata until an upstream operation exists
  (see the capture options in section 5.4).
* **Base block images must be identical on source and target.** containerd
  builds the image's block device or file per host, so two hosts with the same
  OCI image digest do not have the same base. Deltas only work if the base is
  shipped as an artifact or built deterministically (SNAP-VOL-011).
* **Firecracker clones on one host depend on per-process path resolution.**
  Firecracker reopens each block device at the path stored in the snapshot
  and offers no override at load time, so two clones on one host can only use
  different volumes if that path resolves differently in each process
  (SNAP-VOL-007). Flintlock runs Firecracker without the jailer today, so the
  design has to choose a mechanism (open question 8).
* **Cross-host restore needs matching hosts.** Firecracker requires the same
  snapshot format version, CPU vendor, and effective guest-visible CPU
  configuration, and treats host kernel changes as unstable. Because the
  effective configuration depends on BIOS, microcode, kernel, and Firecracker
  version, an update on one side can break compatibility even between hosts
  with identical hardware and CPU template. Cloud Hypervisor gives no
  guarantees. In practice, restore targets need to run the same hardware and
  software as the source host.
* **Clones share guest identity.** Until per-clone network namespaces exist,
  clones that keep the source's MAC and IP need to be kept on separate networks or
  reconfigured by the guest. RNG reseeding depends on VMGenID support in the
  guest kernel, which arm64 guests on Firecracker only get with a backported
  6.1 kernel (SNAP-CLN-006).
* **Guest agent dependency for quiesce only.** Optional quiesce and thaw
  (SNAP-QSC-003) require new guest-agent features. Snapshots without quiesce do
  not depend on the guest agent.
* **Snapshot packages are not encrypted.** Guest memory and the stored spec
  metadata are readable by anyone with access to the package, so access has to
  be restricted with registry and storage permissions until encryption is added
  ([section 5.12](#512-future-work-encryption-informative)).

## 7. Open questions

1. What are the exact `artifactType` and layer media type names?
2. How are host identifiers and host labels for the restore policy defined and
   configured?
3. What is the format of the restore generation counter (SNAP-RST-008), and
   who allocates it when restores happen on different hosts?
4. Which storage backend should volumes use: stay on devmapper, move to the
   containerd `blockfile` snapshotter on a reflink filesystem, or move to a
   read-only base plus writable disk? See
   [0204-root-volume-options.md](0204-root-volume-options.md).
5. How are base block images produced, distributed, cached on hosts, and
   garbage collected? Shipped artifact or deterministic build?
6. What is the encoding of a volume delta (SNAP-VOL-009)?
7. Should we propose an upstream containerd change so that snapshotters can
   take a point-in-time snapshot of an active snapshot
   ([PR #13111](https://github.com/containerd/containerd/pull/13111) is a
   precedent)? That would make the capture tracked by containerd.
8. How is the block device path stored in a Firecracker snapshot made to
   resolve to each clone's own volume (SNAP-VOL-007): a relative path with a
   per-VM working directory, a per-VM mount namespace, or the jailer's chroot?
   Snapshot load has no drive override, and flintlock runs Firecracker without
   the jailer today.
9. How does flintlock obtain the effective guest-visible CPU configuration
   for the compatibility descriptor (SNAP-META-001, SNAP-RST-004)?
   `cpu-template-helper fingerprint dump` boots a throwaway microVM, so it is
   a measurement per host, CPU template, kernel, and Firecracker version that
   could be taken once and cached, invalidated when any of those change.
   Reading the vCPU state from the snapshot state file is an alternative.

## 8. References

* Issue [#204](https://github.com/liquidmetal-dev/flintlock/issues/204)
* [Firecracker snapshotting docs](https://github.com/firecracker-microvm/firecracker/tree/main/docs/snapshotting),
  in particular `snapshot-support.md`,
  [`versioning.md` (CPU model)](https://github.com/firecracker-microvm/firecracker/blob/main/docs/snapshotting/versioning.md#cpu-model)
  and [(device model)](https://github.com/firecracker-microvm/firecracker/blob/main/docs/snapshotting/versioning.md#device-model),
  [`cpu-template-helper.md`](https://github.com/firecracker-microvm/firecracker/blob/main/docs/cpu_templates/cpu-template-helper.md)
  (fingerprint dump and compare),
  `network-for-clones.md`, and
  [`random-for-clones.md` (kernels with VMGenID)](https://github.com/firecracker-microvm/firecracker/blob/main/docs/snapshotting/random-for-clones.md#linux-kernels-with-vmgenid-support)
* [OCI distribution specification](https://github.com/opencontainers/distribution-spec/blob/main/spec.md)
  (manifests and blobs are fetched by repository name and digest)
* [Firecracker `PATCH /drives` docs](https://github.com/firecracker-microvm/firecracker/blob/main/docs/api_requests/patch-block.md),
  [vhost-user block docs](https://github.com/firecracker-microvm/firecracker/blob/main/docs/api_requests/block-vhost-user.md),
  and issue [#4014](https://github.com/firecracker-microvm/firecracker/issues/4014)
  (drive remapping at load)
* [Cloud Hypervisor snapshot and restore](https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/docs/snapshot_restore.md),
  [API](https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/docs/api.md),
  and [disk locking](https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/docs/disk_locking.md)
* containerd [devmapper snapshotter](https://github.com/containerd/containerd/blob/v1.7.35/snapshots/devmapper/snapshotter.go),
  [blockfile snapshotter](https://github.com/containerd/containerd/blob/main/plugins/snapshots/blockfile/blockfile.go),
  and [PR #13111](https://github.com/containerd/containerd/pull/13111)
* Linux [dm-thin documentation](https://github.com/torvalds/linux/blob/master/Documentation/admin-guide/device-mapper/thin-provisioning.rst),
  [`thin_delta(8)`](https://github.com/jthornber/thin-provisioning-tools/blob/main/man8/thin_delta.txt),
  and [`ioctl_ficlone(2)`](https://man7.org/linux/man-pages/man2/ioctl_ficlone.2.html)
* [0204-root-volume-options.md](0204-root-volume-options.md): research behind
  the volume requirements
* [OCI image specification](https://github.com/opencontainers/image-spec) and
  [OCI distribution specification](https://github.com/opencontainers/distribution-spec)
* [ocicrypt](https://github.com/containers/ocicrypt)
* [RFC 2119](https://www.rfc-editor.org/rfc/rfc2119) and
  [RFC 8174](https://www.rfc-editor.org/rfc/rfc8174)
