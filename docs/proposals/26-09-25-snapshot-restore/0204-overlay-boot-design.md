# 0204. Overlay Boot: design for the first Option C milestone

* Status: proposed design
* Date: 2026-09-28
* Authors: @richardcase
* Issue: [#204](https://github.com/liquidmetal-dev/flintlock/issues/204)
* Companions: [requirements](0204-snapshot-restore-requirements.md),
  [options survey](0204-root-volume-options.md),
  [Option B](0204-option-b-blockfile.md),
  [Option C](0204-option-c-ro-base-rw-disk.md),
  decision record `0204-root-volume-decision.md`
  ([#1252](https://github.com/liquidmetal-dev/flintlock/pull/1252), not yet
  merged)

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
by decisions D1 to D15 of the decision record. Where this document departs
from either, the change is listed as a numbered amendment in
[section 13](#13-amendments-to-the-decision-record).

Option C as amended splits into eight pieces of work. This document covers
the first four.

| # | Piece | Depends on | In this document |
| - | ----- | ---------- | ---------------- |
| 1 | Guest artefacts: kernel and initrd | none | yes |
| 2 | Base builder | none | yes |
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
  uses container additional volumes still needs the thin pool.
* The default flip to `overlay` (D6) and writable disk growth.
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
6. `full` mode is unchanged and the existing e2e suite passes without edits.
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
[`mikrolite-images`](https://github.com/liquidmetal-dev/mikrolite-images). The
state of the published images on 2026-09-28, read from the config file each
image carries:

| Image | EROFS | overlayfs | ext4 | initrd | virtio-blk |
| ----- | ----- | --------- | ---- | ------ | ---------- |
| `firecracker-kernel:6.1` | no | yes | yes | yes | yes |
| `firecracker-kernel:5.10` | no | yes | yes | yes | yes |
| `firecracker-kernel:5.10-no-acpi` | no | yes | yes | yes | yes |
| `firecracker-kernel-k8s:6.1` | no | yes | yes | yes | yes |
| `firecracker-kernel-k8s:5.10` | no | yes | yes | yes | yes |
| `cloudhypervisor-kernel-k8s:6.2` | no | yes | yes | yes | yes |
| `cloudhypervisor-kernel:6.2` | no | no | no | no | no |

EROFS is added with a config fragment in each kernel component. The Cloud
Hypervisor kernels also need `CONFIG_MISC_FILESYSTEMS=y`. That repository
builds every kernel without loadable modules and enforces it in the build and
in CI, so the initrd never carries modules.

The bare `cloudhypervisor-kernel:6.2` image cannot boot a flintlock VM in any
mode. Layer M1 fixes it ([section 12.2](#122-stack-in-mikrolite-images)).

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
`flintlockd` sets the `flintlock.root.*` parameters after the spec's own
kernel command line is applied, so a spec cannot override them.

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
8. Delete the contents of the initramfs, `switch_root` to the overlay and
   exec the image's init.

The base and the writable disk are not exposed inside the guest. The overlay
keeps working without their mount points being reachable.

### 4.5 Failure behaviour

On any error the init writes one line with the reason to the console and
exits. The kernel panics, and with the existing `panic=1 reboot=k` parameters
the VMM process exits. The host sees a VM that stopped, with the reason in its
console log. There is no rescue shell.

### 4.6 Initrd build and publication

The init's source lives in `cmd/flintlock-overlay-init/`. It is built with
`CGO_ENABLED=0`. Syscalls sit behind an interface so that parsing, ordering
and failure paths are unit tested without a boot. Packaging into a gzip
compressed cpio archive lives under `hack/` (D3). The release workflow
publishes the initrd as an OCI image tagged with the `flintlockd` version.

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

Everything is on one filesystem, so that renames are atomic and reflinks are
possible.

**Rule.** Every fact about a base or a writable disk is derivable from this
layout plus the VM record. There is no ledger in this milestone
([A7](#13-amendments-to-the-decision-record)). When the snapshot milestones
add one, it is an index that a scan can rebuild, and the backfill for VMs
created before it is the same code as its recovery path.

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

`full` mode does not use this port. It keeps the existing `VolumeMount` step
and `ImageService`.

### 5.3 Package `infrastructure/volume/overlay`

| Unit | Does | Depends on |
| ---- | ---- | ---------- |
| Base builder | Layers to one EROFS file. Writes to `scratch/`, fsyncs, computes the sha256, renames into `bases/` | `LayerSource`, `mkfs.erofs` |
| Base index | Look up by chain ID and profile, record, list references. One `flock` per key, so concurrent creates build once | filesystem |
| Writable disk | Creates a template per size on first use, then `FICLONE`s it per VM | `mkfs.ext4` |
| Sweep | On release, removes bases that no VM links to and that are older than the retention period. Clears `scratch/` at startup | filesystem |

The base index is an internal interface with the filesystem as its only
implementation. A database-backed implementation can replace it without
changes to the builder or the port.

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
([A6](#13-amendments-to-the-decision-record)). The profile `erofs-v1` fixes
every input to the build:

| Input | Value |
| ----- | ----- |
| erofs-utils | 1.9.4, the static binary that provisioning installs |
| Block size | 4096, passed explicitly, because the default is the build host's page size |
| Compression | none ([A4](#13-amendments-to-the-decision-record)) |
| Build time | `-T0 --mkfs-time`: superblock time is zero, files keep the mtimes recorded in the layers |
| UUID | Version 5, derived from the layer diff ID (per layer) or from the chain ID and profile (merged image) |

The build has two steps and never unpacks the image
([A5](#13-amendments-to-the-decision-record)):

```text
# per layer, tar stream on stdin
mkfs.erofs -b4096 --tar=f --aufs -T0 --mkfs-time -U <uuid> <layer>.erofs /dev/stdin

# merge, lowest layer first
mkfs.erofs -b4096 --clean=data --ovlfs-strip=1 -T0 --mkfs-time -U <uuid> \
    <merged>.erofs <layer-1>.erofs ... <layer-n>.erofs
```

The per-layer files are intermediate and are removed after the merge. The
base digest is the sha256 of the merged file.

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

The drive order is decided in three places today: each provider's emission
code and the arithmetic in `core/steps/cloudinit/disk_mount.go`. One function
in `infrastructure/microvm/shared` replaces them. It is exposed through a new
method on the provider port:

```go
// Drives returns the drives the provider will present, in guest order.
Drives(vm *models.MicroVM) ([]models.Drive, error)
```

A `Drive` carries its ID, role, guest device name, read-only flag and, when
the volume status has one, its host path.

| Consumer | Uses |
| -------- | ---- |
| Provider emission | The whole list, in order |
| Kernel parameters | Device names of the base and the writable disk |
| Cloud-init mount step | Device names of the additional volumes (D14) |

This fixes two existing defects in the cloud-init mount step: it counts
virtiofs entries as block devices, and on Cloud Hypervisor it ignores the
cloud-init image at `vdb`.

### 6.2 Provider changes

| | Firecracker | Cloud Hypervisor |
| - | ----------- | ---------------- |
| Drives | `<id>_base` read-only and `<id>_rw`; neither is the root device | `--disk` for the base with `readonly=on`, then the writable disk |
| Initrd | Already passes `initrd_path` | Gains `--initramfs` (D4) |
| Command line | Adds the `flintlock.root.*` parameters | Same, and omits `root=/dev/vda rw` |
| Paths | Relative (`volumes/root.base`, `volumes/root.rw`), with the process working directory set to the VM state directory | Absolute, unchanged |

Firecracker stores the drive path string in its snapshot, and a clone must
resolve that string to its own disk (SNAP-VOL-007). Overlay VMs are therefore
created with relative paths from the start, so that VMs created by this
milestone can be snapshotted by a later one. `full` mode keeps absolute paths.

### 6.3 Application and plan

| Where | Change |
| ----- | ------ |
| `app.CreateMicroVM` | In overlay mode with no initrd in the spec, write the configured default into the spec (D2). The stored spec names the initrd that was used |
| Validation | Reject overlay with a read-only root. Reject overlay on additional volumes. Reject a writable disk size under the minimum |
| Capabilities | Host probes at startup add `volume-overlay` and `volume-full` to each provider's capabilities. A spec that asks for a missing mode is rejected (D7) |
| Create plan | New `OverlayVolumeMount` step calls `VolumeService.Create` and records both mounts and the base digest |
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
| `--overlay-initrd-image` | The published initrd image at the daemon's own version |
| `--overlay-rw-size-mb` | 10240, the devmapper base size used today |
| `--overlay-base-retention` | 24h |
| `--mkfs-erofs-path` | The location provisioning installs to |

## 7. Host provisioning

`flintlock-provision` gains two subcommands and one flag.

### 7.1 `flintlock-provision erofs-utils`

Installs the pinned `mkfs.erofs` as a static binary. The binary is built in
CI from the erofs-utils tag that the profile names, published as a flintlock
release asset, and verified by checksum on download, in the way Firecracker
and Cloud Hypervisor are installed. It is installed under flintlock's own
directory and does not replace a system `mkfs.erofs`.

The distribution package is not used. Ubuntu 24.04 ships erofs-utils 1.7.1,
and the merge step needs 1.8 or later.

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
| e2e | Success criteria 1, 2, 4, 5 and 6 on Firecracker 6.1, Firecracker 5.10 and Cloud Hypervisor 6.2 | KVM, reflink state directory, rebuilt kernels |

Criterion 3 is proven by the integration tests. Criterion 7 is proven by the
unit tests of the host probes and the capability rules.

In CI a missing `mkfs.erofs` fails the integration tests. It does not skip
them.

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

Not yet tested: a real multi-layer image, and a mount on a 5.10 kernel.

## 11. Spikes before the implementation plan is final

1. Build the base of the `mikrolite-images` `ubuntu` image with the merge and
   compare it against an unpack of the same layers.
2. Boot that base on a 5.10 kernel with EROFS enabled.

If either fails, the fallback is D8 close to how it was written: apply the
layers into a scratch directory with containerd's archive package, then run
`mkfs.erofs` on the directory. Only the internals of the base builder change.

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

| Layer | Change |
| ----- | ------ |
| M1 | Fix the bare Cloud Hypervisor kernel: pin the config source and fail the build when the download fails |
| M2 | Enable EROFS in every kernel component. Update the text that says microVMs boot with no initrd |

### 12.3 Stack in `flintlock`

Bottom first.

| Layer | Branch | Change | Note |
| ----- | ------ | ------ | ---- |
| F1 | `overlay/drive-order` | Derive cloud-init device names from the provider's drive order | Fixes `full` mode today. Changes guest device names for Cloud Hypervisor VMs that have additional volumes with mount points; needs a release note |
| F2 | `overlay/ch-initramfs` | Cloud Hypervisor passes the initrd to the VMM | Fixes `full` mode today |
| F3 | `overlay/ch-readonly` | Cloud Hypervisor honours read-only volumes | Fixes `full` mode today |
| F4 | `overlay/api-volume-mode` | Volume mode, base mount and capabilities in the API and models, with validation | Changes `userdocs/` |
| F5 | `overlay/init` | Overlay init and initrd packaging and publication | |
| F6 | `overlay/mkfs-erofs` | CI build of the static `mkfs.erofs`, and `flintlock-provision erofs-utils` | Below F7, whose integration tests need the binary |
| F7 | `overlay/base-builder` | Base builder and `LayerSource` | After the spikes |
| F8 | `overlay/volume-service` | `VolumeService`, writable disks, base index, sweep, settings and host probes | |
| F9 | `overlay/providers` | Overlay drives and kernel parameters in both providers | |
| F10 | `overlay/plan` | Overlay volume step, release step and default initrd | |
| F11 | `overlay/provision-host` | `flintlock-provision state-dir` and the optional thin pool | |
| F12 | `overlay/e2e` | e2e for overlay boot | Needs M2 published |

F1 to F3 can merge as soon as they are approved. Overlay mode becomes usable
when F10 merges.

## 13. Amendments to the decision record

| # | Amends | Change |
| - | ------ | ------ |
| A1 | D3, D5, record section 4 | Kernel work goes to `mikrolite-images`, not `image-builder`. EROFS and its options are built in |
| A2 | D1 | The initrd uses `switch_root`. `pivot_root` does not work from an initramfs root |
| A3 | D5 | The base is limited to EROFS features a 5.10 kernel can mount, and the profile that fixes them is part of the base identity |
| A4 | D8 | No compression in the base. Compressed output depends on the compression library's version as well as on erofs-utils. Bases can be compressed in transit when distribution is built |
| A5 | D8 | The base is built from layer tar streams and a merge, not from a native snapshotter `View`. The native snapshotter copies the whole tree for every layer. The tar route needs no snapshotter and never creates the image's device nodes or setuid files on the host |
| A6 | D8, D11 | The base is keyed by rootfs chain ID and profile, not by image digest. Two manifests with the same layers share one base, and the key does not change when a registry recompresses a layer |
| A7 | Option C sections 4, 5.1 and 6; Option B section 5.3 | No bolt `volumes.db`. [#1251](https://github.com/liquidmetal-dev/flintlock/pull/1251) selects SQLite for the local store and rules out bbolt. Volume records go into that store when the snapshot milestones need them |

D3 is also refined: the init's Go source lives in `cmd/`, where the
repository keeps binary entrypoints, and only the packaging is under `hack/`.

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
