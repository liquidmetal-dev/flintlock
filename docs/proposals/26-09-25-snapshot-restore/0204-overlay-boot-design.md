# 0204. Overlay Boot: design for the first Option C milestone

* Status: proposed design
* Date: 2026-09-28
* Authors: @richardcase
* Issue: [#204](https://github.com/liquidmetal-dev/flintlock/issues/204)
* Companions: [requirements](0204-snapshot-restore-requirements.md),
  [options survey](0204-root-volume-options.md),
  [Option A](0204-option-a-devmapper.md), [Option B](0204-option-b-blockfile.md),
  [Option C](0204-option-c-ro-base-rw-disk.md),
  [decision record](0204-root-volume-decision.md)

## 1. Summary

This document designs the first milestone of Option C: a microVM boots with
its root volume in `overlay` mode. The root volume is presented as two drives,
a read-only EROFS base built from the OCI image and a per-VM writable ext4
disk. An initrd that flintlock builds and publishes assembles an overlayfs
root from the two and hands over to the image's own init.

The milestone contains no snapshot code. It delivers the storage layout that
snapshot capture, restore and clone are later built on, and it proves the
parts with the most unknowns: the guest boot contract, the deterministic base
build and the drive order.

## 2. Relationship to other documents

The design starts from [Option C](0204-option-c-ro-base-rw-disk.md) as amended
by decisions D1 to D15 of the
[decision record](0204-root-volume-decision.md). Where this document departs
from either, the change is listed as a numbered amendment in
[section 13](#13-amendments-to-the-decision-record-and-to-option-c).

Option C as amended splits into eight pieces of work. This document covers
the first four.

| # | Piece | Depends on | In this document |
| - | ----- | ---------- | ---------------- |
| 1 | Guest artefacts: kernel and initrd | none | yes |
| 2 | Base builder | 4, for the pinned `mkfs.erofs` | yes |
| 3 | Overlay boot path | 1, 2 | yes |
| 4 | Host provisioning | none | yes |
| 5 | Base distribution (D10, D11) | 2 | no |
| 6 | Snapshot capture | 3 | no |
| 7 | Restore and clone | 5, 6 | no |
| 8 | Additional volumes (D12), then the default flip (D6) | 3, 6 | no |

## 3. Scope

### 3.1 In scope

| Area | What is built |
| ---- | ------------- |
| Guest kernel | EROFS enabled in the `mikrolite-images` kernels |
| Initrd | A Go init, packed and published as an OCI image, versioned with `flintlockd` |
| Base builder | OCI image to one deterministic EROFS file, cached per host |
| Writable disk | Sparse ext4 template per size, reflinked per VM (D13) |
| API and models | `Volume.mode`, a second mount in the volume status, new capabilities |
| Providers | Two drives for the root volume on both VMMs |
| Plan | Overlay volume step, default initrd (D2), device names from drive order (D14) |
| Provisioning | Reflink state directory, pinned erofs-utils, optional thin pool (D7) |

### 3.2 Out of scope

* Base push and pull, snapshot capture, restore and clone.
* Additional volumes in overlay form (D12). They stay `full`, so a VM that
  uses container additional volumes still needs the thin pool, and it cannot
  be snapshotted by a later milestone (SNAP-VOL-010), because snapshot
  support is chosen at creation.
* The default flip to `overlay` (D6) and writable disk growth.
* The deprecation notice for `full` (D6). It lands with the default flip.
* Enforcing the guest kernel requirement (D5). A spec that pairs `overlay`
  with a kernel without EROFS, such as the 4.19 image, is accepted and fails
  at boot as in section 4.5.
* A read-only root in overlay mode. Validation rejects it.
* arm64.
* Moving the state directory of a host that has running VMs onto a new
  filesystem. The provisioner handles a fresh or drained host.

### 3.3 Success criteria

1. A VM whose root volume has `mode: overlay` reaches running on Firecracker
   and on Cloud Hypervisor, from an unmodified rootfs image.
2. The guest root is overlayfs with an EROFS lower and an ext4 upper.
3. Building the base twice from one image gives the same sha256.
4. A second VM from the same image reuses the base and gets its own writable
   disk.
5. Deleting a VM removes its writable disk and links, and leaves the base.
6. `full` mode is unchanged from `main` after F1 to F3 (section 12.3), and
   the existing e2e suite passes without edits.
7. A host with no thin pool rejects `full` VMs with a capability error.

## 4. Guest contract

The contract is the interface between `flintlockd` and the initrd. A spec
that names its own initrd (D2) must honour it.

### 4.1 Kernel requirements

Built in, not modules: `CONFIG_EROFS_FS`, `CONFIG_EROFS_FS_XATTR`,
`CONFIG_EROFS_FS_POSIX_ACL`, `CONFIG_EROFS_FS_SECURITY`, `CONFIG_EROFS_FS_ZIP`,
`CONFIG_OVERLAY_FS`, `CONFIG_EXT4_FS`, `CONFIG_VIRTIO_BLK`, `CONFIG_DEVTMPFS`,
`CONFIG_BLK_DEV_INITRD` and `CONFIG_RD_GZIP`.

The xattr options are required because file capabilities are stored in
xattrs. `CONFIG_EROFS_FS_ZIP` is not used by this milestone (section 5.4) and
is enabled so that compression can be turned on later without a kernel
rebuild.

Guest kernels are built in
[`mikrolite-images`](https://github.com/liquidmetal-dev/mikrolite-images).
EROFS is added there with a config fragment in each kernel component
([section 12.2](#122-stack-in-mikrolite-images)). The Cloud Hypervisor kernels
also need `CONFIG_MISC_FILESYSTEMS=y`. That repository builds every kernel
without loadable modules and enforces it in the build and in CI, so the
initrd never carries modules.

The state of the published images, read from the config file each image
carries:

| Image | 2026-09-28 | 2026-09-29 | Build with EROFS |
| ----- | ---------- | ---------- | ---------------- |
| `firecracker-kernel:6.1` | no EROFS | EROFS | `6.1-20260929-1349` |
| `firecracker-kernel:5.10` | no EROFS | EROFS | `5.10-20260929-1349` |
| `firecracker-kernel:5.10-no-acpi` | no EROFS | EROFS | `5.10-no-acpi-20260929-1349` |
| `cloudhypervisor-kernel:6.2` | no EROFS, virtio-blk, ext4, overlayfs or initrd support | EROFS | `6.2-20260929-1411` |
| `firecracker-kernel-k8s:6.1` | no EROFS | EROFS | `6.1-20260929-1426` |
| `firecracker-kernel-k8s:5.10` | no EROFS | EROFS | `5.10-20260929-1426` |
| `cloudhypervisor-kernel-k8s:6.2` | no EROFS | EROFS | `6.2-20260929-1426` |

Apart from the bare Cloud Hypervisor kernel before 2026-09-29, every image
has overlayfs, ext4, virtio-blk, devtmpfs and initrd support built in.

Each build is published under two tags: one that moves (`5.10`) and one that
names the build and is never overwritten (`5.10-20260929-1349`).

### 4.2 Drive order

| Position | Firecracker | Cloud Hypervisor |
| -------- | ----------- | ---------------- |
| 1 | root base, read-only | root base, read-only |
| 2 | root writable disk | root writable disk |
| 3 | additional volumes | cloud-init image |
| 4 onward | | additional volumes |

No drive is flagged as the root device, so Firecracker adds no `root=`
parameter. Cloud Hypervisor's default `root=/dev/vda rw` is omitted in overlay
mode.

### 4.3 Kernel parameters

| Parameter | Meaning |
| --------- | ------- |
| `flintlock.root.contract=1` | Contract version. The init refuses a version it does not know |
| `flintlock.root.base=/dev/vda` | Read-only EROFS base device |
| `flintlock.root.rw=/dev/vdb` | Writable ext4 device |
| `init=` | Optional. The image's init to exec. Default `/sbin/init` |

`flintlockd` computes the device names from the drive order it emits, so the
init hard-codes no order. The init reads `/proc/cmdline`, because the kernel
does not pass parameters that contain a dot to init's environment.

The kernel command line is a map of unique keys
(`internal/config/kernel_cmd_line.go`). `flintlockd` sets the
`flintlock.root.*` keys after the spec's entries are merged, so a spec value
for one of them is replaced and the init never sees a key twice. The order of
the parameters in the emitted line is not defined.

When an initramfs carries `/init`, the kernel runs it and ignores `init=`,
which is why the init can read `init=` as the name of the image's init.
`rdinit=` is different: it replaces `/init` itself. Validation rejects a spec
that sets `rdinit` in overlay mode, because it would bypass the contract.

### 4.4 Init behaviour

1. Mount devtmpfs on `/dev` and proc on `/proc`.
2. Parse and validate the parameters.
3. Wait up to 10 seconds for both devices to appear.
4. Mount the base as EROFS, read-only.
5. Mount the writable disk as ext4. Create `upper` and `work` on it if they
   are absent.
6. Mount the overlay.
7. Write one line to the console that names the contract version, both
   devices and their filesystem types.
8. `switch_root` to the overlay, which empties the initramfs, and exec the
   image's init.

The mount points of the base and the writable disk are not reachable from
the overlay root; the block devices stay visible in `/dev`. The overlay keeps
working without the mount points being reachable.

### 4.5 Failure behaviour

On any error the init writes one line with the reason to the console and
exits. The kernel panics, and with the `panic=1 reboot=k` parameters the VMM
process exits. Both providers hard-code those parameters, and a spec can
replace a value but not remove a key (section 4.3), so they are always
present. The host sees a VM that stopped, with the reason in its console log.
There is no rescue shell.

### 4.6 Initrd build and publication

The init's source lives in `cmd/flintlock-overlay-init/`. It is built with
`CGO_ENABLED=0`. Syscalls sit behind an interface so that parsing, ordering
and failure paths are unit tested without a boot. Packaging into a gzip
compressed cpio archive lives under `hack/` (D3). The release workflow gains
a step that publishes the initrd as an OCI image,
`ghcr.io/liquidmetal-dev/flintlock-overlay-init`, tagged with the `flintlockd`
version. The workflow publishes no OCI image today, only the GoReleaser
binaries and packages, so the registry login and the push are new.

## 5. Host design

### 5.1 On-host layout

```text
<StateRootDir>/
├── vm/<ns>/<name>/<uid>/volumes/
│   ├── root.base -> ../../../../../bases/<sha256>.img    # symlink
│   └── root.rw                                           # reflink of a template
├── bases/
│   ├── <sha256>.img                                      # mode 0400
│   └── by-source/<chain-id>/<profile> -> ../../<sha256>.img
├── templates/rw-<size-mb>.img
└── scratch/                                              # build temp
```

The chain ID is the OCI rootfs chain ID of the image: the hash chain of its
uncompressed layer digests. It identifies the unpacked tree and does not
change when a registry recompresses a layer. The profile names the fixed set
of build inputs (section 5.4). Together they are the key of a base.

Everything is on one filesystem, so that renames are atomic and reflinks are
possible.

**Rule.** Every fact about a base or a writable disk is derivable from this
layout plus the VM record. There is no ledger in this milestone
([A7](#13-amendments-to-the-decision-record-and-to-option-c)). When the
snapshot milestones add one, it is an index that a scan can rebuild, and the
backfill for VMs created before it is the same code as its recovery path.

### 5.2 Port

```go
// core/ports/services.go
type VolumeService interface {
    // Create provisions the drives of an overlay volume for a VM. It is
    // idempotent.
    Create(ctx context.Context, in VolumeCreateInput) (*VolumeMounts, error)
    // Release removes a VM's writable disk and links. It is idempotent.
    Release(ctx context.Context, ref VolumeRef) error
}
```

`VolumeCreateInput` carries the VM ID, the volume ID, the image reference and
the size. `VolumeMounts` carries the base mount, the writable mount and the
base digest. `Capture`, `Base`, `Delta` and `Restore` from
[Option B section 5.1](0204-option-b-blockfile.md#51-new-port-volumeservice)
are added by later milestones.

`Create` is idempotent in this sense. The create plan runs again on every
daemon restart and resync, and a writable disk that exists may hold guest
writes, so if `volumes/root.rw` exists it is kept and never reflinked over.
If `root.base` exists and resolves to a base with the VM's key, it is kept.
The plan step's `ShouldDo` is false when the status records both mounts and
both files are present.

`full` mode does not use this port. It keeps the existing `VolumeMount` step
and `ImageService`.

### 5.3 Package `infrastructure/volume/overlay`

| Unit | Does | Depends on |
| ---- | ---- | ---------- |
| Base builder | Layers to one EROFS file. Writes to `scratch/`, fsyncs, computes the sha256, renames into `bases/` | `LayerSource`, `mkfs.erofs` |
| Base index | Look up by chain ID and profile, record, list references. One `flock` per key, so concurrent creates build once | filesystem |
| Writable disk | Creates a template per size on first use, in `scratch/` with an fsync and a rename under a per-size `flock`, then `FICLONE`s it per VM | `mkfs.ext4` |
| Sweep | On release, removes bases that no VM links to and whose last use is older than the retention period. Clears `scratch/` at startup | filesystem |

The base index is an internal interface with the filesystem as its only
implementation. A database-backed implementation can replace it without
changes to the builder or the port.

A base's last use is the modification time of its `by-source` link, which
`Create` refreshes on every use. The base file's own mtime is never touched,
so section 9 can use it as evidence that the base is unchanged. The sweep
finds references by scanning `vm/*/*/*/volumes/root.base`. It holds the
per-key lock while it scans and unlinks, so a `Create` that is linking the
same base waits (section 8).

Locks are `flock`s, not in-process mutexes, because
`internal/command/run/run.go` initialises two sets of ports.

`LayerSource` is an interface the package defines:

```go
type LayerSource interface {
    // Layers pulls the image if needed and returns its rootfs chain ID and
    // one uncompressed tar stream per layer, lowest first.
    Layers(ctx context.Context, in LayerInput) (*ImageLayers, error)
}
```

It is implemented in `infrastructure/containerd`, which reads layers from the
content store under the VM's lease. The overlay package imports no containerd
code, so its tests run on tar fixtures.

### 5.4 Base build

The base is keyed by the image's rootfs chain ID and a profile name
([A6](#13-amendments-to-the-decision-record-and-to-option-c)). The profile
`erofs-v1` fixes every input to the build:

| Input | Value |
| ----- | ----- |
| erofs-utils | 1.9.4, the static binary that provisioning installs |
| Block size | 4096, passed explicitly, because the default is the build host's page size |
| Compression | none ([A4](#13-amendments-to-the-decision-record-and-to-option-c)) |
| Build time | `-T0 --mkfs-time`: superblock time is zero, files keep the mtimes recorded in the layers |
| UUID | Version 5 in the namespace `uuid5(NAMESPACE_URL, "https://liquidmetal.dev/flintlock/erofs-base")`, from the layer diff ID (per layer) or from the chain ID and profile (merged image) |

The build has two steps and never unpacks the image
([A5](#13-amendments-to-the-decision-record-and-to-option-c)):

```text
# per layer, tar stream on stdin
mkfs.erofs -b4096 --tar=f --aufs -T0 --mkfs-time -U <uuid> <layer>.erofs /dev/stdin

# merge, lowest layer first
mkfs.erofs -b4096 --clean=data --ovlfs-strip=1 -T0 --mkfs-time -U <uuid> \
    <merged>.erofs <layer-1>.erofs ... <layer-n>.erofs
```

The per-layer files are intermediate and are removed after the merge. The
base digest is the sha256 of the merged file. D8's `--sort` keeps its
default, `path`: both values are deterministic for a fixed tar stream, and
the stream is fixed by the layer digest.

The builder checks the version that `mkfs.erofs` reports against the profile
before every build. A change to any input in the table is a new profile name
and therefore a new base identity.

### 5.5 Writable disk

`templates/rw-<size-mb>.img` is created on first use of a size: a sparse file
of that size formatted with
`mkfs.ext4 -E lazy_itable_init=0,lazy_journal_init=0 -U <fixed> -L flintlock-rw`.
Lazy initialisation is off so that the guest does not write inode tables into
every clone. `Create` reflinks the template to the VM's `volumes/root.rw`.

The size comes from `Volume.Size`, with the daemon default when the spec
gives none (D13). The minimum is 64 MB.

### 5.6 Models and API

* `Volume.mode`: `full` (default) or `overlay`. Proto enum `VolumeMode`.
* `VolumeStatus` gains `BaseMount` and `BaseDigest`. The existing `Mount`
  holds the writable disk.
* New mount type `file`. The existing `dev` and `hostpath` mean a block
  device and a directory.
* New capabilities `volume-full` and `volume-overlay`.

## 6. Providers and plan

### 6.1 One source for drive order

Since F1 ([#1270](https://github.com/liquidmetal-dev/flintlock/pull/1270),
merged 2026-10-03) the drive order has one source, `BuildDrives` in
`infrastructure/microvm/shared/drives.go`, exposed through the provider port:

```go
// Drives returns the drives the provider will present, in guest order.
Drives(vm *models.MicroVM) (models.Drives, error)
```

A `Drive` (`core/models/drive.go`) carries its ID, role (`root`, `cloud-init`
or `volume`), volume ID, guest device name, read-only flag and, when the
volume status has one, its host path. Both providers emit from the list, and
the cloud-init mount step takes its device names from it (D14). F1 also fixed
two defects in that step: it counted virtiofs entries as block devices, and
on Cloud Hypervisor it ignored the cloud-init image at `vdb`.

This milestone adds two roles, `root-base` and `root-rw`, and one consumer:

| Consumer | Uses | State |
| -------- | ---- | ----- |
| Provider emission | The whole list, in order | exists |
| Cloud-init mount step | Device names of the additional volumes | exists |
| Kernel parameters | Device names of the base and the writable disk | new |

### 6.2 Provider changes

| | Firecracker | Cloud Hypervisor |
| - | ----------- | ---------------- |
| Drives | `<id>_base` read-only and `<id>_rw`; neither is the root device | `--disk` for the base with `readonly=on`, then the writable disk |
| Initrd | Already passes `initrd_path` | Passes `--initramfs` since F2 (D4) |
| Command line | Adds the `flintlock.root.*` parameters | Same, and omits `root=/dev/vda rw` |
| Paths | Relative (`volumes/root.base`, `volumes/root.rw`), with the process working directory set to the VM state directory | Absolute, unchanged |
| Guest TRIM | Not enabled; `discard` keeps its default | Not enabled; `sparse` keeps its default |

Firecracker stores the drive path string in its snapshot, and a clone must
resolve that string to its own disk (SNAP-VOL-007). Overlay VMs are therefore
created with relative paths from the start, so that VMs created by this
milestone can be snapshotted by a later one. `full` mode keeps absolute paths.

For the same reason guest TRIM (Option C section 5.3) is a decision and not
an omission: a drive's flags are part of the device configuration a snapshot
restores with. It stays at the VMM defaults for VMs created by this milestone
([A9](#13-amendments-to-the-decision-record-and-to-option-c)). Enabling it
for VMs created later does not affect existing VMs.

### 6.3 Application and plan

| Where | Change |
| ----- | ------ |
| `app.CreateMicroVM` | In overlay mode with no initrd in the spec, write the configured default into the spec (D2). The stored spec names the initrd that was used |
| Validation | Reject overlay with a read-only root. Reject overlay on additional volumes. Reject a writable disk size under the minimum |
| Capabilities | Each provider's `Capabilities()` is a static list today. Host probes run once at startup and their result is given to both providers, which append `volume-overlay` and `volume-full`. `checkProviderCapabilities` gains a check of the volume mode with a fixed error per missing mode (D7) |
| Create plan | New `OverlayVolumeMount` step calls `VolumeService.Create` and records both mounts and the base digest. Its `ShouldDo` is defined in section 5.2 |
| Delete plan | New `VolumeRelease` step before the directory delete. It also runs the base sweep |

Host probes:

| Capability | Probe |
| ---------- | ----- |
| `volume-overlay` | A real `FICLONE` in the state directory succeeds; `mkfs.erofs` is present and reports the profile's version; `mkfs.ext4` is present |
| `volume-full` | containerd reports the devmapper snapshotter as loaded |

A failed probe is logged with its reason.

### 6.4 Daemon settings

| Flag | Default |
| ---- | ------- |
| `--overlay-initrd-image` | The published initrd image at the daemon's own version. A build whose version is not a release tag has no default, and overlay mode is rejected with an error that names the flag until it is set |
| `--overlay-rw-size-mb` | 10240, the 10 GB `base_image_size` that provisioning gives devmapper today |
| `--overlay-base-retention` | 24h |
| `--mkfs-erofs-path` | The location provisioning installs to |

## 7. Host provisioning

`flintlock-provision` gains two subcommands and one flag.

### 7.1 `flintlock-provision erofs-utils`

Installs the pinned `mkfs.erofs` as a static binary. The binary is built in
CI from the erofs-utils tag that the profile names, published as a flintlock
release asset with its sha256, and verified against that checksum on
download. The provisioner does not checksum Firecracker or Cloud Hypervisor
today; this is the first install that is verified. The binary is installed
under flintlock's own directory and does not replace a system `mkfs.erofs`.

The distribution package is not used. Ubuntu 24.04 ships erofs-utils 1.7.1,
which predates the multi-source merge of section 5.4, and the inline xattr
ordering fix for reproducible builds arrived in 1.9.3.

### 7.2 `flintlock-provision state-dir`

| Case | Action |
| ---- | ------ |
| The directory is already on XFS with reflink, or on Btrfs | Verify with a `FICLONE` probe and change nothing |
| A block device or logical volume is given | Format as XFS with `reflink=1`, mount at the state directory, persist the mount |
| Development and e2e | A sparse file on a loop device, formatted the same way |

XFS is the filesystem the provisioner creates. Btrfs is accepted where it
already exists. `xfsprogs` is added to the apt package list.

### 7.3 Optional thin pool

| Piece | Change |
| ----- | ------ |
| `flintlock-provision all` | New flag to skip the thin pool. Without it, behaviour is unchanged |
| containerd config template | The devmapper section is written only when a pool exists |

Kernel and initrd images keep using the native snapshotter, so they work on a
host with no pool.

### 7.4 Resulting host states

| Host has | Advertises | Accepts |
| -------- | ---------- | ------- |
| Thin pool only (every host today) | `volume-full` | `full` only |
| Thin pool and reflink state directory | both | both modes, and overlay roots with `full` additional volumes |
| Reflink state directory only | `volume-overlay` | overlay roots with no container additional volumes |

## 8. Failure handling

| Step | Failure | Handling |
| ---- | ------- | -------- |
| Base build | `mkfs.erofs` exits non-zero | Remove scratch files and return its stderr in the error. The reconciler's retry applies, then the VM is marked failed |
| Base build | A layer has a hardlink to a file in a lower layer | Same path. The error names the layer and the entry |
| Base build | Daemon crash during the build | The kernel drops the `flock`. The startup sweep clears `scratch/` |
| Base build | Two creates for one image at once | The second waits on the lock, then finds the finished base |
| Base build | Disk full | Remove scratch files and fail |
| Template | Two creates for one size at once | The second waits on the per-size lock, then reflinks the finished template |
| Writable disk | `FICLONE` is not supported | Fail with a configuration error. There is no fallback to a full copy, because it would hide a misprovisioned host |
| Sweep | A base is removed while a create is linking it | The sweep takes the same per-key lock and rechecks references under it |
| Release | Files already gone | Success |
| Guest init | Any error | Section 4.5 |

**Accepted risk.** A base is not checksummed again each time a VM links to
it, because that costs a full read for every VM. The file is mode 0400 and
named by its digest. Verification on pull and a periodic check arrive with
base distribution.

## 9. Testing

| Level | Covers | Needs |
| ----- | ------ | ----- |
| Unit | Init parameter parsing, contract refusal, device timeout and mount order; drive order for both providers and both modes; validation and capability rules; plan steps with a mocked `VolumeService`; provider argument building | nothing |
| Integration | Base determinism; the merged image equals containerd's own unpack of the same layers by path, mode, owner, size, content hash, link groups and xattrs; index locking | pinned `mkfs.erofs`; root for the unpack comparison |
| e2e | Success criteria 1, 2, 4, 5 and 6 on Firecracker 6.1, Firecracker 5.10 and Cloud Hypervisor 6.2 | KVM, a reflink state directory, the published kernels |

Criterion 3 is proven by the integration tests. Criterion 7 is proven by the
unit tests of the host probes and the capability rules.

In CI a missing `mkfs.erofs` fails the integration tests. It does not skip
them. The integration job builds the pinned erofs-utils from source, or
restores it from the actions cache, because F7's tests run before a release
that carries the binary exists.

The e2e runner has no reflink filesystem today, and the e2e harness starts
`flintlockd` with the default state directory. The hosted e2e workflow
creates the loop-backed XFS of section 7.2 and mounts it at that directory
before `flintlockd` starts.

The rootfs image for e2e is the unmodified `ubuntu` image from
`mikrolite-images`. Cloud Hypervisor e2e depends on
[#1264](https://github.com/liquidmetal-dev/flintlock/pull/1264).

Each criterion is proven from evidence other than the status flintlock
reports:

| Criterion | Evidence |
| --------- | -------- |
| The root is the overlay | The init's console line. The writable disk's allocated blocks exceed the template's after boot |
| The base is untouched | Its sha256 still equals its file name after the VM has run |
| The base is reused | The second VM's `root.base` resolves to the same inode, and the base's modification time has not changed |
| Delete | The writable disk is gone and the base is present |
| cloud-init copes with an overlay root | Its completion line appears on the console |

The last row is there because cloud-init's `resizefs` module will find an
overlay root. It is expected to skip it with a warning. The test confirms
that.

## 10. Experiments

All with erofs-utils 1.9.4, uncompressed, on 2026-09-28.

**X-1, merge of two layers.** The second layer overrides a file and whites
out a file and a directory. The merged image extracted correctly with the
layer files absent: override applied, whiteouts applied, a hardlink pair kept,
a 500 kB file byte-identical. Two builds gave the same sha256. Without
`--clean=data` the merge fails.

**X-2, merge of three layers, harder cases.** Built four times, with 1, 1, 8
and 24 worker threads.

| Case | Result |
| ---- | ------ |
| Opaque directory | Lower contents removed, new contents kept, the `overlay.opaque` marker absent from the merged image |
| Directory whited out, then recreated by a later layer | Correct, with the later layer's mode |
| `security.capability` xattr | Preserved |
| Setuid bit, non-root uid and gid, character device, symlink | Preserved |
| Hardlink within a layer | Preserved, link count 2 |
| sha256 across the four builds | Identical |
| Incompatible feature flags | None. Compatible flags: `sb_csum`, `mtime`, `xattr_filter` |
| Hardlink whose target is in a lower layer | The build fails with "No such file or directory", exit code 1 |

Layers produced from an overlayfs diff do not contain the last case, because
overlayfs copies the link target up into the same layer. The OCI image
specification does not forbid it.

**X-3, single source.** A directory and a flattened tar of the same tree each
built deterministically. The two differ from each other, as expected.

**X-4, merge of a real image.** `ghcr.io/liquidmetal-dev/ubuntu:24.04`
(manifest `sha256:49c7b918d4442db3d0e26a2c1d5683a5ebaaeca746b37d722872cf4e04927ac2`,
three layers, 449 MiB as tar). The build, three layer images and the merge,
took under 0.6 seconds on Btrfs with the layers in the page cache. It gave a
base of 444792832 bytes with no incompatible feature flags. Four builds gave
the same sha256.

The rule for this experiment was set before the run: read the base back
through a loop mount, compare it with docker's unpack of the same image, and
fail on any difference. The loop mount needs root on the host, which was not
available. The base was read back with `fsck.erofs --xattrs --extract`
instead, as root in a container.

20443 paths were compared. None was missing or extra, and none differed in
type, mode, owner, size, content, modification time (regular files), symlink
target, device numbers or hardlink group. Seven more paths, which docker
overwrites in every container (`dev`, `proc`, `sys`, `etc/hostname`,
`etc/hosts`, `etc/resolv.conf`, `etc/mtab`), were compared layer against base
directly and are equal.

Two paths differed: `usr/bin/ping` and
`usr/lib/x86_64-linux-gnu/gstreamer1.0/gstreamer-1.0/gst-ptp-helper` have a
`security.capability` xattr in docker's unpack and none in the extracted
tree. By the rule, this fails the experiment.

The result is overridden here. That is a judgment, not a measurement. The
reason is that the fault is in the reader and not in the base:
`fsck.erofs --xattrs` sets the xattr on the extracted file and then calls
`lchown`, and Linux drops `security.capability` when a file is chowned. The
base holds exactly two `security.capability` entries, on these two files,
with docker's values byte for byte, and X-5 shows that a guest kernel returns
them.

The comparison through a kernel mount has not been run. It is part of the
integration test of section 9, which cannot use `fsck.erofs --extract` for
xattrs.

**X-5, overlay root under Firecracker v1.16.1.** The base of X-4 as the
first drive, read-only, a 2 GiB ext4 disk as the second, neither flagged as
root device, and a shell init in an initrd. The init follows steps 1 and 3 to
6 of section 4.4 with fixed device names, and then calls `switch_root`. It
does not parse parameters, write the contract line or empty the initramfs.
Kernels 5.10.245 and 6.1.0, built locally from `mikrolite-images` with the
EROFS fragment of section 4.1.

| Check | 5.10.245 | 6.1.0 |
| ----- | -------- | ----- |
| The kernel mounts the base as EROFS | yes | yes |
| The overlay is assembled and systemd starts from it | yes, login prompt, no failed unit | yes, login prompt, no failed unit |
| `getcap` in the guest, through the overlay, on the two files of X-4 | equal to docker's | equal to docker's |
| The guest wrote to the upper | `etc`, `var`, `tmp`; nothing under `usr` | the same |
| sha256 of the base after the boot | unchanged | unchanged |

`getcap` returned `cap_net_raw=ep` for `ping` and
`cap_net_bind_service,cap_net_admin,cap_sys_nice=ep` for `gst-ptp-helper`.

Not shown by X-5:

* cloud-init did not run, most likely because no datasource was attached.
  There is no evidence yet of how it treats an overlay root.
* Cloud Hypervisor was not booted.

On 5.10 only, the kernel logs a failed probe (error -16) of each virtio-mmio
device. The block devices then attach with the right sizes.

**X-6, the published kernels.** The four Firecracker kernels published from
`mikrolite-images` at `6d7757c` on 2026-09-29. The configuration of the
published `firecracker-kernel:5.10` is equal to that of the locally built
kernel of X-5. Each was booted as in X-5, with the same base, initrd and
drives, for 60 seconds.

| Kernel image | Linux | Overlay root, login prompt, no failed unit | `getcap` equal to docker's | Upper written |
| ------------ | ----- | ------------------------------------------ | -------------------------- | ------------- |
| `firecracker-kernel:5.10-20260929-1349` | 5.10.245 | yes | yes | yes |
| `firecracker-kernel:6.1-20260929-1349` | 6.1.0 | yes | yes | yes |
| `firecracker-kernel-k8s:5.10-20260929-1426` | 5.10.199 | yes | yes | yes |
| `firecracker-kernel-k8s:6.1-20260929-1426` | 6.1.0 | yes | yes | yes |

The sha256 of the base was unchanged after the boots. "Upper written" means
that the allocated blocks of the writable disk grew and that the upper holds
`etc`, `var` and `tmp`.

Two of the four kernels log the failed probe of the virtio-mmio devices that
X-5 saw. They are the two that are built with both `CONFIG_ACPI` and
`CONFIG_VIRTIO_MMIO_CMDLINE_DEVICES`:

| Kernel image | `ACPI` | `VIRTIO_MMIO_CMDLINE_DEVICES` | Failed probe |
| ------------ | ------ | ----------------------------- | ------------ |
| `firecracker-kernel:5.10` | y | y | yes |
| `firecracker-kernel:6.1` | y | not set | no |
| `firecracker-kernel-k8s:5.10` | not set | y | no |
| `firecracker-kernel-k8s:6.1` | y | y | yes |

This fits a device that is announced twice, on the command line and through
ACPI, with the second registration refused as busy (error -16). That reading
was not confirmed beyond the table. The drives attach in every case.

The two published Cloud Hypervisor kernels, `cloudhypervisor-kernel` and
`cloudhypervisor-kernel-k8s`, have the options of section 4.1 in their
configuration. They were not booted.

## 11. Outcome of the spikes

Two questions were open before the base builder could be planned: whether
the layer merge is correct for a real image, and whether a 5.10 guest kernel
boots an overlay root on the result. Experiments X-4 and X-5, the two spikes,
answer both with yes.

The first yes rests on the override in X-4. The comparison read the base
with `fsck.erofs` and not through a kernel mount, and the file capabilities
were confirmed from the bytes of the base and by `getcap` in the guest. The
comparison through a kernel mount is still to be run, in the integration test
of section 9.

Amendment A5 stands: the base is built by layer merge, without an unpack. The
other way, applying the layers into a scratch directory with containerd's
archive package and running `mkfs.erofs` on the directory, is not used.

The merge cannot build an image that has a layer with a hardlink into a lower
layer (X-2). Section 8 fails the build with an error that names the layer and
the entry. Whether flintlock offers another way to build such an image is
decided in the plan of the base builder.

## 12. Delivery

Every pull request has one concern, and every commit compiles with
`make test` passing.

### 12.1 Stacked pull requests

The work is delivered as GitHub stacked pull requests, managed with the
official `gh stack` extension
([github/gh-stack](https://github.com/github/gh-stack)). Each layer is one
branch and one pull request. Its base is the layer below, so a reviewer sees
only that layer's diff. The bottom layer is based on `main`.

| Rule | Reason |
| ---- | ------ |
| Branches are pushed to the repository itself, not to a fork | A pull request can only take a branch of the same repository as its base |
| Layers are created with `gh stack init` and `gh stack add`, and opened with `gh stack submit` | `submit` sets each base and links the pull requests as one stack on GitHub |
| A change to a lower layer is followed by `gh stack rebase` or `gh stack sync` | The layers above must contain the new tip of the layer below |
| Every layer compiles and passes `make test` and `make lint` at its own head | CI runs on each layer separately |
| Layers merge from the bottom with `gh stack merge <pr>` | It merges every layer up to the chosen one, or none |
| Every pull request title is a Conventional Commit header, and no description has an agent footer | The rules in `AGENTS.md` apply to each layer |

Layers that are useful without the rest sit at the bottom of the stack, so
they can merge while the layers above are still in review.

Two effects of the repository's CI on a stack:

* `test`, `lint`, `commitlint`, `pr_size` and `pr_type` run on every pull
  request whatever its base, so every layer is checked.
* `test-docs` runs only for pull requests whose base is `main`. A layer that
  changes `userdocs/` gets that check when it becomes the bottom of the
  stack.

This design document is its own pull request and is not a layer of either
stack.

### 12.2 Stack in `mikrolite-images`

Both changes edit the Cloud Hypervisor kernel's Dockerfile and Makefile, so
they are stacked.

| Layer | Change | State |
| ----- | ------ | ----- |
| M1 | Fix the bare Cloud Hypervisor kernel: pin the config source and fail the build when the download fails | merged 2026-09-29, [mikrolite-images#37](https://github.com/liquidmetal-dev/mikrolite-images/pull/37) |
| M2 | Enable EROFS in every kernel component. Update the text that says microVMs boot with no initrd | merged 2026-09-29, [mikrolite-images#38](https://github.com/liquidmetal-dev/mikrolite-images/pull/38) |

All seven kernels were published the same day (section 4.1). Still open in
that repository:

* `kernel-fc` takes its base configuration from the `main` branch of
  Firecracker and does not pin it. The kernels published on 2026-09-29 carry
  that base as it was on the day. Compared with the images published before,
  they differ by more than EROFS.

### 12.3 Stack in `flintlock`

Bottom first.

| Layer | Branch | Change | Note |
| ----- | ------ | ------ | ---- |
| F1 | `overlay/drive-order` | Derive cloud-init device names from the provider's drive order | Merged 2026-10-03, [#1270](https://github.com/liquidmetal-dev/flintlock/pull/1270). Changes guest device names for Cloud Hypervisor VMs that have additional volumes with mount points; needs a release note |
| F2 | `overlay/ch-initramfs` | Cloud Hypervisor passes the initrd to the VMM | Merged 2026-10-03, [#1271](https://github.com/liquidmetal-dev/flintlock/pull/1271) |
| F3 | `overlay/ch-readonly` | Cloud Hypervisor honours read-only volumes | Merged 2026-10-03, [#1272](https://github.com/liquidmetal-dev/flintlock/pull/1272) |
| F4 | `overlay/api-volume-mode` | Volume mode, base mount and capabilities in the API and models, with validation | Changes `userdocs/` |
| F5 | `overlay/init` | Overlay init and initrd packaging and publication | |
| F6 | `overlay/mkfs-erofs` | CI build of the static `mkfs.erofs`, and `flintlock-provision erofs-utils` | Below F7, whose integration tests need the binary |
| F7 | `overlay/base-builder` | Base builder and `LayerSource` | After the spikes of section 11 |
| F8 | `overlay/volume-service` | `VolumeService`, writable disks, base index, sweep, settings and host probes | |
| F9 | `overlay/providers` | Overlay drives and kernel parameters in both providers | |
| F10 | `overlay/plan` | Overlay volume step, release step and default initrd | |
| F11 | `overlay/provision-host` | `flintlock-provision state-dir` and the optional thin pool | |
| F12 | `overlay/e2e` | e2e for overlay boot | Needs F11 for the state directory on the runner; the Cloud Hypervisor e2e of [#1264](https://github.com/liquidmetal-dev/flintlock/pull/1264) is merged |

F1 to F3 are merged. Overlay mode becomes usable when F10 and F11 are
merged: F10 completes the create path, and F11 gives a host the reflink
state directory that the `volume-overlay` probe needs.

## 13. Amendments to the decision record and to Option C

| # | Amends | Change |
| - | ------ | ------ |
| A1 | D3, D5, record section 4 | Kernel work goes to `mikrolite-images`, which builds the published kernels, not `image-builder`. EROFS and its options are built in, because those kernels carry no modules |
| A2 | D1 | The initrd uses `switch_root`. On the 5.10 and 6.1 guest kernels `pivot_root` does not work from an initramfs root, whose rootfs cannot be unmounted. Newer kernels lift that; the design does not depend on it |
| A3 | D5, D8, D11 | The base is limited to EROFS features a 5.10 kernel can mount, and the profile that fixes them is part of the base identity |
| A4 | D8 | No compression in the base. Compressed output depends on the compression library's version as well as on erofs-utils. Bases can be compressed in transit when distribution is built |
| A5 | D8 | The base is built from layer tar streams and a merge, not from a native snapshotter `View`. The native snapshotter copies the whole tree for every layer. The tar route needs no snapshotter and never creates the image's device nodes or setuid files on the host |
| A6 | D8, D11, Option C section 8 | The base is keyed by rootfs chain ID and profile, not by image digest, and the filesystem UUIDs derive from the diff IDs and from that key rather than from the image digest. Two manifests with the same layers share one base, and the key does not change when a registry recompresses a layer |
| A7 | Option C sections 4, 5.1, 6 and 8; Option B section 5.3 | No bolt `volumes.db`. [#1251](https://github.com/liquidmetal-dev/flintlock/pull/1251) selects SQLite for the local store and rules out bbolt. Volume records go into that store when the snapshot milestones need them |
| A8 | Option C section 5.5 | erofs-utils is a pinned static binary that the provisioner installs, not the distribution package. Ubuntu 24.04 ships 1.7.1, which lacks the merge step |
| A9 | Option C section 5.3 | Guest TRIM is not enabled on the writable disk. Drive flags are part of the device configuration a snapshot restores with, so the choice is recorded now; VMs created later can differ |
| A10 | D3 | The init's Go source lives in `cmd/`, where the repository keeps binary entrypoints. Only the packaging is under `hack/` |

## 14. References

* erofs-utils: <https://git.kernel.org/pub/scm/linux/kernel/git/xiang/erofs-utils.git/tree/man/mkfs.erofs.1>,
  <https://git.kernel.org/pub/scm/linux/kernel/git/xiang/erofs-utils.git/tree/ChangeLog>
* Kernel: <https://github.com/torvalds/linux/blob/master/Documentation/filesystems/erofs.rst>,
  <https://github.com/torvalds/linux/blob/master/Documentation/filesystems/overlayfs.rst>,
  <https://github.com/torvalds/linux/blob/master/Documentation/filesystems/ramfs-rootfs-initramfs.rst>,
  <https://github.com/torvalds/linux/blob/master/Documentation/admin-guide/kernel-parameters.rst>
* Guest kernels: <https://github.com/liquidmetal-dev/mikrolite-images>
* Reference init: <https://github.com/firecracker-microvm/firecracker-containerd/blob/main/tools/image-builder/files_debootstrap/sbin/overlay-init>
* Flintlock: `core/plans/microvm_create_update.go`,
  `core/steps/cloudinit/disk_mount.go`, `core/steps/runtime/volume_mount.go`,
  `core/ports/services.go`, `core/models/volumes.go`,
  `infrastructure/microvm/firecracker/config.go`,
  `infrastructure/microvm/cloudhypervisor/create.go`,
  `infrastructure/containerd/image_service.go`, `internal/provision/`
