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
| Snapshot | The point-in-time state of a microVM: VMM device state, guest memory, and writable volume changes. Also the flintlock API resource that tracks producing it. |
| Snapshot package | The OCI artifact that holds a snapshot and its metadata. |
| Source VM | The microVM a snapshot was taken from. |
| Restored VM | A microVM created from a snapshot package. |
| Clone | One of several restored VMs created from the same snapshot package. |
| Compatibility descriptor | Plaintext metadata in the snapshot package that describes the VMM, host, and images the snapshot depends on. |
| Quiesce | Flushing and freezing the guest's writable filesystems so that the on-disk state is consistent with guest memory. |
| Thaw | Reversing a quiesce. |
| Writable volume | A root or additional volume that is not `is_read_only`. |

## 4. VMM capability summary (informative)

| | Firecracker | Cloud Hypervisor |
| - | ----------- | ---------------- |
| Pause / resume | `PATCH /vm {"state": "Paused" \| "Resumed"}` | `vm.pause` / `vm.resume` |
| Create snapshot | `PUT /snapshot/create` (`snapshot_path`, `mem_file_path`, `snapshot_type`, `sync_snapshot_files`); VM has to be paused | `vm.snapshot` to a `file://` destination directory; VM has to be paused |
| Restore | `PUT /snapshot/load` on a fresh process, before boot | `--restore source_url=...` at launch, or `vm.restore` on an empty VMM |
| Files produced | VM state file (with CRC) and guest memory file | `config.json`, `state.json`, `memory-ranges` (mode 0600) |
| Memory backends | `File` (lazy, `MAP_PRIVATE`) or `Uffd` (external page-fault handler) | `copy` (default, eager), `ondemand` (userfaultfd), `copyonwrite` (shared page cache) |
| Diff snapshots | Developer preview | Not supported |
| Version rules | Each binary supports exactly one snapshot format version; host kernel changes are "unstable"; same CPU model needed | No documented guarantees |
| Not captured | Disk contents, MMDS data store, network connections, logger/metrics config; vsock is reset | Disk contents |
| Host resource remapping | `network_overrides`, `vsock_override` | `net_fds`, or editing `config.json` |
| Uniqueness | Always-on VMGenID; secure only if each snapshot is resumed once | Not documented |

Both VMMs expect disks, TAP devices, and sockets to exist at the same paths at
restore time unless remapped.

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
| SNAP-CRT-005 | Snapshot creation MUST follow this order: quiesce (SNAP-QSC), pause, VMM snapshot, point-in-time capture of writable volumes (SNAP-VOL), then resume or stop the source VM. |
| SNAP-CRT-006 | The client MUST be able to choose whether the source VM is resumed or stopped after the snapshot. The default MUST be to resume. |
| SNAP-CRT-007 | Compression, packaging, and pushing MUST take place after the source VM has been resumed or stopped, so that they do not add to the pause time. |
| SNAP-CRT-008 | Snapshot creation MUST be asynchronous: the request MUST return a snapshot identifier, and the client MUST be able to observe progress and the final result. |
| SNAP-CRT-009 | Flintlock MUST produce only full snapshots. |
| SNAP-CRT-010 | Flintlock MUST NOT allow more than one snapshot of the same microVM to be in progress at a time. |
| SNAP-CRT-011 | If snapshot creation fails at any point after the pause, flintlock MUST return the source VM to its prior running state (thawed and resumed) unless the client asked for it to be stopped. |

### 5.3 Quiesce (SNAP-QSC)

Because volume contents are captured separately from memory (SNAP-VOL), the
guest has to flush and freeze its writable filesystems first. Otherwise the
restored guest's memory (page cache, filesystem journal) would not match the
restored disks.

| ID | Requirement |
| -- | ----------- |
| SNAP-QSC-001 | If a microVM has any writable volume, flintlock MUST quiesce the guest before pausing it. |
| SNAP-QSC-002 | Quiescing MUST be performed through the guest agent. A snapshot request for a microVM with writable volumes MUST be rejected if the microVM was not created with `allow_guest_agent`, unless the client sets the force flag (SNAP-QSC-005). |
| SNAP-QSC-003 | The guest agent MUST provide dedicated freeze and thaw operations. Freeze MUST find the guest's writable filesystems itself, sync them, and freeze them (for example with `FIFREEZE`). This is a dependency on the guest-agent project. |
| SNAP-QSC-004 | The freeze operation MUST take a timeout after which the guest agent thaws the filesystems on its own, so that a guest is never left frozen if flintlock fails. |
| SNAP-QSC-005 | If quiescing fails, the snapshot MUST fail, unless the client set an explicit force flag. A snapshot taken with the force flag MUST be marked as not quiesced in its metadata (SNAP-META-001). |
| SNAP-QSC-006 | Flintlock MUST thaw the source VM after the VMM snapshot and volume capture, whether they succeeded or failed, if the source VM is resumed. |
| SNAP-QSC-007 | A restored VM resumes with its filesystems frozen. Flintlock MUST thaw a restored VM through the guest agent after resuming it, and MUST report the restored VM as failed if thawing does not succeed within a configurable timeout. |

### 5.4 Volumes (SNAP-VOL)

| ID | Requirement |
| -- | ----------- |
| SNAP-VOL-001 | The snapshot package MUST contain the changes made to each writable volume since it was created from its image. The changes MUST be captured as a file-level diff against the image, stored as an OCI layer (tar format) that can be applied to a fresh unpack of the image on any host. Block-level diffs MUST NOT be used, because an image's block layout is not the same on different hosts. |
| SNAP-VOL-002 | Read-only volumes, the kernel, and the initrd MUST NOT be copied into the snapshot package. They MUST be referenced by image digest (SNAP-PKG-006). |
| SNAP-VOL-003 | The source VM MUST remain paused until the writable volumes have been captured at the same point in time as the VMM snapshot. |
| SNAP-VOL-004 | Flintlock SHOULD use the capture mechanism with the shortest pause that the VMM and snapshotter support (see the capture options below). When the mechanism's pause grows with the size of the volume or filesystem, flintlock SHOULD log a warning. |
| SNAP-VOL-005 | Flintlock SHOULD support a configurable maximum pause duration. If the capture would exceed it, flintlock SHOULD abort the snapshot and resume the source VM. |
| SNAP-VOL-006 | On restore, flintlock MUST recreate each writable volume by mounting its image (fetched by digest) and applying the captured diff on top, before starting the VMM. |
| SNAP-VOL-007 | Restored volumes MUST be presented to the VMM at the paths and with the device ordering the snapshot expects, remapping paths where the VMM supports it. |
| SNAP-VOL-008 | After the capture, the source VM's volumes MUST remain usable by the source VM and MUST remain correctly tracked by containerd, so that the source VM can be snapshotted again and deleted normally. |

#### Capture options (informative)

With the devmapper snapshotter, a writable volume (including the root volume)
is an *active* containerd snapshot whose parent is the committed image
snapshot. containerd (v1.7) has no operation that takes a point-in-time copy
of an active snapshot: `Prepare` and `View` require a committed parent, and
`Commit` deactivates the device, which a running VM is still using. dm-thin
itself can snapshot an active device instantly, but containerd does not expose
this.

The design can choose between these mechanisms:

| Mechanism | Pause | Works across hosts | VMMs | Notes |
| --------- | ----- | ------------------ | ---- | ----- |
| Stop the source VM, then `Commit` and diff against the image with containerd's differ | None (the VM is stopped) | Yes | Both | Only when the client asks for the source VM to be stopped (SNAP-CRT-006). |
| Commit and swap: while paused, `Commit` the active snapshot, `Prepare` a new active snapshot from it, point the VM's drive at the new device, resume; then diff the committed snapshot against the image | About instant | Yes | Firecracker (`PATCH /drives/{id}`) | Cloud Hypervisor cannot swap a root disk. Grows the containerd snapshot chain by one per snapshot. |
| Mount the frozen filesystem read-only on the host (`ro,noload`) while paused, and diff it against a `View` of the image | Grows with the filesystem tree | Yes | Both | Only safe while the guest is both frozen and paused. |
| Copy the whole block device while paused | Grows with allocated disk size | Yes (no image needed on restore) | Both | Large packages; does not meet SNAP-VOL-001 as a diff and would need its own layer type. |
| Create a dm-thin snapshot directly in containerd's pool (`dmsetup message ... create_snap`) | About instant | Yes, after a file diff | Both | Creates devices containerd does not track, so device IDs can collide and garbage collection does not see them; conflicts with SNAP-VOL-008. |
| Block-level diff (`thin_delta`) against the image device | Grows with changed data | No | Both | Conflicts with SNAP-VOL-001. |
| Extend containerd's devmapper snapshotter to allow an active parent | About instant | Yes | Both | Needs an upstream change; see open questions. |

A likely design is to use stop-and-commit when the source VM is stopped,
commit-and-swap for Firecracker, and the read-only host mount for Cloud
Hypervisor, until containerd supports snapshots of active devices.

### 5.5 Packaging (SNAP-PKG)

| ID | Requirement |
| -- | ----------- |
| SNAP-PKG-001 | A snapshot package MUST be an OCI artifact as defined by the [OCI image specification](https://github.com/opencontainers/image-spec), with a flintlock-specific `artifactType` and media types. |
| SNAP-PKG-002 | The package's config blob MUST hold the snapshot metadata (SNAP-META). |
| SNAP-PKG-003 | The VMM state files, guest memory, and each writable volume diff MUST each be stored in separate layers, identified by media type and annotations. |
| SNAP-PKG-004 | Guest memory layers MUST be compressed with zstd. |
| SNAP-PKG-005 | Guest memory SHOULD be split into multiple layers of bounded size, so that push and pull can run in parallel and resume after interruption. |
| SNAP-PKG-006 | The package MUST record the digest of every image the snapshot depends on (kernel, initrd, and all volumes). Image references by tag alone MUST NOT be used. |
| SNAP-PKG-007 | The package SHOULD link to the manifests of the images it depends on, through an OCI index or the subject/referrers mechanism, so that tools copying the snapshot can copy those images too and registry garbage collection does not remove them. |
| SNAP-PKG-008 | The package MUST carry a flintlock snapshot package format version. Flintlock MUST refuse to restore a package whose major format version it does not support. |
| SNAP-PKG-009 | Flintlock MUST support pushing a snapshot package to an OCI registry. |
| SNAP-PKG-010 | Registry credentials MUST be configured on the flintlockd host, per registry (for example with a docker `config.json`-style file). The same credentials MUST be used to pull snapshot packages and their images on restore. Credentials MUST NOT be passed in API requests. |
| SNAP-PKG-011 | Flintlock MUST support storing a snapshot package locally, as an [OCI image layout](https://github.com/opencontainers/image-spec/blob/main/image-layout.md) directory under the flintlock state directory, so it can be copied with standard tools such as `oras`. |
| SNAP-PKG-012 | Flintlock MUST verify the digest of every blob it reads from a snapshot package. |

### 5.6 Metadata (SNAP-META)

| ID | Requirement |
| -- | ----------- |
| SNAP-META-001 | The package MUST contain a plaintext compatibility descriptor with at least: VMM type; exact VMM version; VMM snapshot format version (Firecracker); flintlock version; package format version; host CPU architecture; CPU vendor, model, and feature flags, or the CPU template used; host kernel version; interrupt controller version (arm64 GIC); vCPU count and memory size; digests of the images the snapshot depends on; whether the snapshot was quiesced; creation time; source VM UID, name, and namespace; client-supplied labels; and the restore policy (SNAP-SEC-002), if any. |
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
| SNAP-RST-004 | Before starting the VMM, flintlock MUST check the compatibility descriptor against the host and MUST reject the restore, with an error naming the mismatch, if any of these differ: VMM type, CPU architecture, interrupt controller version, or (Firecracker) snapshot format version, or (Cloud Hypervisor) exact VMM version. |
| SNAP-RST-005 | Flintlock SHOULD also reject the restore if the CPU vendor, model, feature flags (or CPU template), or host kernel version differ. The client MAY set a force flag to skip the checks in this requirement, but not those in SNAP-RST-004. |
| SNAP-RST-006 | Flintlock MUST check the restore policy (SNAP-SEC-002) and signature (SNAP-SEC-004) before pulling the guest memory, VMM state, or volume layers. |
| SNAP-RST-007 | By default, flintlock MUST give the restored VM the source VM's metadata. The client MAY supply replacement metadata. |
| SNAP-RST-008 | Flintlock MUST add restore markers to the restored VM's metadata: the snapshot identifier, the restored VM's UID, and a restore generation counter that is unique for each restore of the same snapshot. |
| SNAP-RST-009 | The client MAY choose the memory restore mode. The default MUST be `copy` for Cloud Hypervisor and `File` for Firecracker. Support for Firecracker `Uffd` MAY be added later. |
| SNAP-RST-010 | When a lazy memory mode is used (Firecracker `File` or `Uffd`, Cloud Hypervisor `ondemand` or `copyonwrite`), flintlock MUST keep the memory file in place and unchanged, protected by a containerd lease or equivalent, until every VM using it has been deleted. |
| SNAP-RST-011 | A restored VM MUST be managed like any other microVM: it MUST be reconciled, reported through `GetMicroVM` and `ListMicroVMs`, and deleted through `DeleteMicroVM`, including clean-up of its restored volumes and memory files. |
| SNAP-RST-012 | A restored VM MUST be resumed after restore, and MUST be reported as created only after it has been resumed and thawed (SNAP-QSC-007). |
| SNAP-RST-013 | If a restore fails, flintlock MUST remove any partial state it created (VMM process, volumes, network devices, local copies of the package) and report the microVM as failed. |

### 5.8 Clones (SNAP-CLN)

Restoring one snapshot more than once duplicates everything held in guest
memory: identifiers, tokens, keys, RNG state, and the guest's MAC and IP
addresses. Firecracker documents that the only secure pattern is to resume each
snapshot exactly once.

| ID | Requirement |
| -- | ----------- |
| SNAP-CLN-001 | Flintlock MUST allow the same snapshot package to be restored more than once, on the same or different hosts. |
| SNAP-CLN-002 | Each restored VM MUST have a unique UID, unique host-side network devices, and a unique vsock path. |
| SNAP-CLN-003 | Flintlock MUST NOT attempt to change guest-side identity (hostname, MAC and IP inside the guest, machine ID). The guest is responsible for this, using the restore markers (SNAP-RST-008) and any replacement metadata. |
| SNAP-CLN-004 | Snapshots of microVMs with macvtap interfaces MUST NOT be restored more than once in this version. |
| SNAP-CLN-005 | The user documentation MUST warn that clones share RNG state, secrets, and guest network identity, and MUST describe the mitigations. |
| SNAP-CLN-006 | Guests that will be cloned SHOULD use a kernel with VMGenID support (Linux 5.18 or later) so that the kernel RNG is reseeded on restore. |

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
| SNAP-SEC-004 | Flintlock SHOULD support signing snapshot packages (for example with cosign or notation). When a package carries a restore policy, flintlockd MUST verify its signature against public keys configured on the host and MUST reject the restore if verification fails. |
| SNAP-SEC-005 | A flintlockd host MAY be configured to require a valid signature for every restore. |
| SNAP-SEC-006 | The user documentation MUST state that snapshot packages are not encrypted, that they contain the guest's secrets (in guest memory and in the stored spec metadata), that the restore policy is not a security boundary on its own, and that access to packages has to be restricted with registry and storage permissions. |
| SNAP-SEC-007 | Flintlock SHOULD support configurable limits on local snapshot storage and on the number of concurrent snapshot operations, to prevent snapshots exhausting host resources. |
| SNAP-SEC-008 | The package format SHOULD allow encryption to be added in a later version without a new major package format version (SNAP-PKG-008), for example by staying compatible with the [ocicrypt](https://github.com/containers/ocicrypt) encrypted layer media types. |

### 5.10 API (SNAP-API)

| ID | Requirement |
| -- | ----------- |
| SNAP-API-001 | Snapshots MUST be exposed through a new `SnapshotService` in `snapshot.services.api.v1alpha1`, with operations to create a snapshot, get a snapshot, list snapshots, stream a list of snapshots, and delete a snapshot. Each operation MUST have a grpc-gateway REST mapping, following the pattern in `api/services/microvm/v1alpha1/microvms.proto`. |
| SNAP-API-002 | A snapshot MUST be a first-class resource with an identifier, the source VM, the destination (registry reference or local store), labels, and a status. The status MUST include a phase (pending, quiescing, capturing, packaging, pushing, ready, failed), an error message on failure, and on success the package's reference and digest. |
| SNAP-API-003 | `CreateMicroVM` MUST accept either a microVM spec (as today) or a snapshot source. A snapshot source MUST contain the package reference, the overrides allowed by SNAP-RST-002, the memory restore mode, and the force flag (SNAP-RST-005). |
| SNAP-API-004 | All API changes MUST be backwards compatible with existing `v1alpha1` clients. |
| SNAP-API-005 | Deleting a snapshot MUST remove its record and any local package content. Flintlock MUST NOT delete snapshot packages from a remote registry. |
| SNAP-API-006 | Deleting a snapshot MUST NOT affect restored VMs that are using its memory file (SNAP-RST-010); that content MUST be released only when those VMs have been deleted. |
| SNAP-API-007 | `ServerInfo` SHOULD report the VMM types and versions available on the host, and the host's compatibility descriptor fields, so clients can pick a compatible host before restoring. |

### 5.11 Operations (SNAP-OPS)

| ID | Requirement |
| -- | ----------- |
| SNAP-OPS-001 | Flintlock MUST publish events for snapshot phase changes and for restores. |
| SNAP-OPS-002 | Flintlock SHOULD expose metrics for pause duration, snapshot size, packaging time, push time, and restore time. |
| SNAP-OPS-003 | If a snapshot operation fails, flintlock MUST remove partial local artifacts. |
| SNAP-OPS-004 | If flintlockd restarts while a snapshot is in progress, it MUST mark the snapshot as failed, remove partial artifacts, and thaw and resume the source VM if it is still frozen or paused (unless the client asked for it to be stopped). |
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
* **Pause time depends on the volume capture mechanism.** Until containerd can
  snapshot an active device, only Firecracker (commit and swap) or a stopped
  source VM get an almost instant capture. Cloud Hypervisor VMs that resume
  after the snapshot stay paused while their filesystems are diffed (see the
  capture options in section 5.4).
* **Cross-host restore needs matching hosts.** Firecracker requires the same
  CPU model and snapshot format version, and treats host kernel changes as
  unstable. Cloud Hypervisor gives no guarantees. In practice, restore targets
  need to run the same hardware and software as the source host.
* **Clones share guest identity.** Until per-clone network namespaces exist,
  clones that keep the source's MAC and IP need to be kept on separate networks or
  reconfigured by the guest.
* **Guest agent dependency.** Quiesce and thaw (SNAP-QSC-003) require new
  guest-agent features.
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
4. Should we propose an upstream containerd change so that the devmapper
   snapshotter can take a point-in-time snapshot of an active device? That
   would give an instant, VMM-independent volume capture (see the capture
   options in section 5.4).

## 8. References

* Issue [#204](https://github.com/liquidmetal-dev/flintlock/issues/204)
* [Firecracker snapshotting docs](https://github.com/firecracker-microvm/firecracker/tree/main/docs/snapshotting),
  in particular `snapshot-support.md`, `versioning.md`, `network-for-clones.md`,
  and `random-for-clones.md`
* [Cloud Hypervisor snapshot and restore](https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/docs/snapshot_restore.md)
* [OCI image specification](https://github.com/opencontainers/image-spec) and
  [OCI distribution specification](https://github.com/opencontainers/distribution-spec)
* [ocicrypt](https://github.com/containers/ocicrypt)
* [RFC 2119](https://www.rfc-editor.org/rfc/rfc2119) and
  [RFC 8174](https://www.rfc-editor.org/rfc/rfc8174)
