# 0204. Root Volume Options for Snapshot & Restore

* Status: informative
* Date: 2026-09-25
* Authors: @richardcase
* Issue: [#204](https://github.com/liquidmetal-dev/flintlock/issues/204)
* Companion to: [0204-snapshot-restore-requirements.md](0204-snapshot-restore-requirements.md)

## 1. Purpose and scope

This document records the research behind the volume requirements (SNAP-VOL) in
the snapshot and restore proposal. It answers one question:

> When a running microVM is snapshotted, how should its root volume (and any
> other mounted volume) be captured, packaged, and recreated on another
> flintlock host, possibly more than once (clones)?

It compares staying on containerd's devmapper snapshotter with moving to other
storage backends. It does not recommend a direction; that decision is left to
review. Live migration and diff (incremental) snapshots are out of scope, as in
the proposal.

Every claim below cites the source that owns it: project source code, official
documentation, kernel documentation, or man pages. Flintlock code is cited by
path and line as of commit `68533c7`.

## 2. How flintlock provisions volumes today

### 2.1 Port and containerd implementation

The `ImageService` port
([`core/ports/services.go:87-116`](../../core/ports/services.go)) has `Pull`,
`PullAndMount`, `Exists`, and `IsMounted`. It has no unmount, remove, commit, or
export operation.

`PullAndMount`
([`infrastructure/containerd/image_service.go:61-80`](../../infrastructure/containerd/image_service.go))
wraps the context in the configured containerd namespace, takes the VM's lease,
pulls the image, picks a snapshotter by use, and calls `snapshotAndMount`
(lines 156-219), which:

* unpacks the image with that snapshotter if needed;
* sets the parent to `identity.ChainID(image.RootFS)`;
* calls `Prepare(ctx, key, parent, labels)` the first time, or `Mounts` on
  later reconciles.

Every volume, including read-only ones, is therefore a writable *active*
containerd snapshot whose parent is the committed image snapshot. `View` is not
used. The snapshot key is `flintlock/<vmid>/<volume-id>`
([`snapshot.go:10-12`](../../infrastructure/containerd/snapshot.go)), with
`kernel` and `initrd` as the usage IDs for those images.

There is one containerd lease per VM, named `flintlock/<vmid>`
([`lease.go`](../../infrastructure/containerd/lease.go)). The lease also holds
the persisted VM spec. Volumes are never removed individually: `ReleaseLease`
([`repo.go:191-200`](../../infrastructure/containerd/repo.go)) deletes the
lease and containerd's garbage collector removes the snapshots later.

### 2.2 Snapshotter selection

* Defaults ([`pkg/defaults/defaults.go`](../../pkg/defaults/defaults.go)):
  `devmapper` for volumes, `native` for kernel and initrd.
* The kernel snapshotter has a flag; the volume snapshotter does not, and is
  hardcoded in
  [`internal/inject/wire.go:73-80`](../../internal/inject/wire.go).
* `supportedSnapshotters` in
  [`infrastructure/containerd/config.go`](../../infrastructure/containerd/config.go)
  is `overlayfs,native,devmapper`.
* Mount conversion
  ([`convert.go:14-38`](../../infrastructure/containerd/convert.go)) chooses
  the flintlock mount type by snapshotter *name*: `devmapper` gives a `dev`
  mount whose source is the `/dev/mapper/<pool>-snap-N` path, `native` gives a
  `hostpath` directory, and `overlayfs` returns an empty path (a stub).
  Anything else is an error.

### 2.3 Volumes as VMM drives

The device path is passed straight to the VMM. Nothing on the host mounts the
filesystem.

* Firecracker
  ([`infrastructure/microvm/firecracker/config.go:71-101`](../../infrastructure/microvm/firecracker/config.go)):
  `path_on_host` is the mount source, `is_read_only` comes from the spec, cache
  type is `Unsafe`.
* Cloud Hypervisor
  ([`infrastructure/microvm/cloudhypervisor/create.go:159-186`](../../infrastructure/microvm/cloudhypervisor/create.go)):
  `--disk path=<source>` for the root and each additional volume, plus the
  generated cloud-init image. `is_read_only` is not passed for either.

Kernel and initrd are resolved as files inside a directory mount
([`infrastructure/microvm/shared/imagefile.go`](../../infrastructure/microvm/shared/imagefile.go)),
so they depend on a directory-producing snapshotter such as `native`.

### 2.4 Related observations

* `Size` and `PartitionID` on a volume are converted from the API but never
  read; every devmapper volume gets the pool's `base_image_size`.
* The thin pool is created by
  [`hack/scripts/devpool.sh`](../../hack/scripts/devpool.sh) (loop-backed) or
  [`hack/scripts/direct_lvm.sh`](../../hack/scripts/direct_lvm.sh), and by the
  Go provisioner in [`internal/provision/`](../../internal/provision/), which
  writes `base_image_size="10GB"` and `discard_blocks=true` into the containerd
  config.
* `go.mod` pins the containerd client at v1.7.35.

## 3. Restored volumes must be byte-identical

The first draft of the proposal required a *file-level* diff of each writable
volume, applied to a fresh unpack of the image on the target host, because the
image's block layout differs between hosts. That premise is correct about
layout but wrong about what a memory restore needs.

### 3.1 Why

A VMM snapshot captures guest memory. That memory contains the guest kernel's
in-memory filesystem state for every mounted filesystem: cached inodes and their
extent maps, block-group descriptors and allocation bitmaps, journal position,
directory entries, and the page cache. All of these refer to *block addresses*
on the disk the guest was using. Freezing the filesystem (`FIFREEZE`) flushes
dirty data and metadata to disk; it does not drop these caches.

If the restored guest is handed a disk with the same *files* but a different
block layout, its cached metadata points at the wrong blocks. Reads return the
wrong data and writes land in the wrong place, corrupting the filesystem.

containerd's devmapper snapshotter builds every base image on every host by
running `mkfs` on a fresh thin device and untarring the layers into it
(`createSnapshot` in
[`snapshots/devmapper/snapshotter.go`](https://github.com/containerd/containerd/blob/v1.7.35/snapshots/devmapper/snapshotter.go)).
Two hosts that pull the same image digest therefore end up with different
block allocations, inode numbers, and filesystem UUIDs. A fresh unpack on the
target is *not* the disk the guest saw. The same is true after applying a tar
layer on top of it: `archive.Apply` rebuilds file contents, not block layout
([`core/diff/apply/apply.go`](https://github.com/containerd/containerd/blob/main/core/diff/apply/apply.go)).

### 3.2 Consequences

* Every volume the guest has *mounted* at snapshot time, writable or read-only,
  must be presented on restore with byte-identical contents. The kernel and
  initrd are exempt: they are loaded into memory at boot and not read again.
* Capture must be at block level. File-level diffs remain a valid way to build
  a new *image* from a stopped VM's disk for a later cold boot, but not for
  resuming a memory snapshot.
* Guest quiesce is no longer required for consistency. Both VMMs pause the
  guest and drain block I/O before a snapshot (section 4), so a block capture
  taken while the VMM is paused is exactly what the guest's memory expects,
  in the same way that a VM that was never stopped is consistent with its own
  disk. Quiesce remains useful for *application* consistency (for example
  flushing a database) but is optional.
* A restored VM does not need a thaw step unless it was quiesced.

### 3.3 The base image must be portable too

If the package carries only the *delta* against the base (the committed image
snapshot's block content), the target host must hold a base that is byte-for-
byte the same as the one on the source host. Because containerd builds bases
per host, that is only true if either:

1. the base block image is exported once from the source host and distributed
   as its own content-addressed artifact, and target hosts create volumes from
   that artifact rather than from a local unpack; or
2. the base block image is built deterministically from the OCI image. EROFS
   images are reproducible by design
   ([`mkfs.erofs`](https://github.com/erofs/erofs-utils/blob/master/man/mkfs.erofs.1)
   with a fixed timestamp); reproducible ext4 needs a fixed UUID, hash seed,
   timestamps, and untar order, which the devmapper snapshotter does not do.

This applies whichever storage backend is chosen. Shipping the whole volume
instead of a delta removes the dependency at the cost of package size.

## 4. VMM facts

### 4.1 Firecracker

Sources: [`firecracker.yaml`](https://github.com/firecracker-microvm/firecracker/blob/main/src/firecracker/swagger/firecracker.yaml),
[`snapshot-support.md`](https://github.com/firecracker-microvm/firecracker/blob/main/docs/snapshotting/snapshot-support.md),
[`patch-block.md`](https://github.com/firecracker-microvm/firecracker/blob/main/docs/api_requests/patch-block.md),
[`block-io-engine.md`](https://github.com/firecracker-microvm/firecracker/blob/main/docs/api_requests/block-io-engine.md),
[`block-vhost-user.md`](https://github.com/firecracker-microvm/firecracker/blob/main/docs/api_requests/block-vhost-user.md).

* **Block devices are raw files only.** `Drive` has `path_on_host`,
  `is_read_only`, `cache_type`, `io_engine` (Sync or Async; Async is developer
  preview and needs a host kernel newer than 5.10.51), `rate_limiter`. There is
  no qcow2 or other format support.
* **Snapshot create** (`PUT /snapshot/create`) requires the VM to be paused.
  "Block device backing files are always fsync'd." The docs then say the
  backing files "should be backed up externally by the user": disk capture is
  the integrator's job, done while paused.
* **Snapshot load** (`PUT /snapshot/load`) accepts `network_overrides` and
  `vsock_override` only. Disks must be "accessible at the same relative paths"
  as when the snapshot was taken. Drive remapping is an open request
  ([#4014](https://github.com/firecracker-microvm/firecracker/issues/4014)).
  Flintlock therefore needs a stable, per-VM drive path (for example a symlink
  or device node under the VM's state directory) that is repointed on the
  target before load.
* **`PATCH /drives/{id}`** can change `path_on_host` and the rate limiter
  post-boot, but only when the "guest did not mount the device", and the update
  is not atomic. It is designed for hot-plug of stub drives, not for swapping a
  mounted root disk.
* **vhost-user-block** is developer preview and "Snapshotting is not supported
  for microVMs that have vhost-user devices configured." This closes the route
  of serving qcow2 or other formats to Firecracker from an external daemon.

### 4.2 Cloud Hypervisor

Sources: [`snapshot_restore.md`](https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/docs/snapshot_restore.md),
[`api.md`](https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/docs/api.md),
[`cloud-hypervisor.yaml`](https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/vmm/src/api/openapi/cloud-hypervisor.yaml),
[`hotplug.md`](https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/docs/hotplug.md),
[`disk_locking.md`](https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/docs/disk_locking.md),
[`vmm/src/config.rs`](https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/vmm/src/config.rs),
[`block/src/formats/qcow/backing.rs`](https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/block/src/formats/qcow/backing.rs),
[`vhost_user_block/src/lib.rs`](https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/vhost_user_block/src/lib.rs).

* **Formats.** `--disk` accepts `image_type=raw|qcow2|vhd|vhdx|vmdk`,
  `backing_files=on|off` (default off), and `sparse=on|off`. qcow2 backing
  chains (raw or qcow2 backing, nested) are followed only with
  `backing_files=on`; relative backing paths are resolved from the directory of
  the image that names them. Disk resize is rejected when backing files are in
  use.
* **Snapshot** (`vm.snapshot`) requires a paused VM and writes `config.json`,
  `state.json`, and `memory-ranges`. There is no disk-snapshot API. For
  snapshots that keep the source running, the docs say to "copy the disk images
  while the VM is still paused, and restore against those copies".
* **Restore** (`vm.restore` or `--restore`) takes `source_url`, `prefault`,
  `memory_restore_mode` (`copy`, `ondemand`, `copyonwrite`), `resume`,
  `zone_updates`, and `net_fds`. There is no disk override, but `config.json`
  is "stored in a human readable format so that it could be modified between
  the snapshot and restore phases"; rewriting `disks[].path` is the supported
  way to point at a different device. `copyonwrite` lets clones share one
  memory file.
* **Hot-plug.** `vm.add-disk` and `vm.remove-device` work on a booted VM. There
  is no way to change a mounted root disk's path in place.
* **Locks.** Cloud Hypervisor takes OFD advisory locks on disk images, so a
  restore cannot open a file the source VM still has open.
* **`vhost_user_block`** opens its `path` as a plain aligned file (raw only).

### 4.3 Summary

| | Firecracker | Cloud Hypervisor |
| - | ----------- | ---------------- |
| Disk formats | raw | raw, qcow2 (backing files opt-in), vhd, vhdx, vmdk |
| Drain and sync at snapshot | pause drains; create fsyncs backing files | pause; copy disks while paused |
| Disk path override at restore | none; same relative paths | edit `config.json` |
| Swap mounted root on a live VM | no (`PATCH /drives` needs an unmounted device) | no (only add/remove device) |
| External block backend | vhost-user-block, but disables snapshots | vhost-user-block (raw) |
| Shared memory for clones | `File` (`MAP_PRIVATE`) or `Uffd` | `copyonwrite` |

## 5. containerd facts

### 5.1 devmapper snapshotter

Sources (v1.7.35; `main` has moved the code to `plugins/snapshots/devmapper`
with the same behaviour):
[`snapshotter.go`](https://github.com/containerd/containerd/blob/v1.7.35/snapshots/devmapper/snapshotter.go),
[`pool_device.go`](https://github.com/containerd/containerd/blob/v1.7.35/snapshots/devmapper/pool_device.go),
[`metadata.go`](https://github.com/containerd/containerd/blob/v1.7.35/snapshots/devmapper/metadata.go),
[`config.go`](https://github.com/containerd/containerd/blob/v1.7.35/snapshots/devmapper/config.go),
[`storage/bolt.go`](https://github.com/containerd/containerd/blob/v1.7.35/snapshots/storage/bolt.go).

* **Prepare and View** both go through `storage.CreateSnapshot`, which rejects
  a parent that is not committed (`parent %q is not committed snapshot`). With
  no parent, they send `create_thin`, activate the device, and run `mkfs`
  (ext4 by default, with `-E nodiscard,lazy_itable_init=0,lazy_journal_init=0`
  unless `fs_options` is set). With a parent, `CreateSnapshotDevice` suspends
  the parent if it is active ("must be suspend before taking a snapshot to
  avoid corruption"), sends `create_snap <new> <parent>`, resumes the parent,
  and activates the new device. View differs only in the `ro` mount option.
* **Commit** records usage, removes the active key, then suspends, resumes, and
  deactivates the device with deferred removal (working around
  [#4234](https://github.com/containerd/containerd/issues/4234)). A device the
  VM still holds open disappears when the VM closes it. **Committing a running
  VM's volume is not a capture mechanism.**
* **Remove** refuses snapshots with children, runs `blkdiscard` when
  `discard_blocks` is set, deactivates, and sends `delete <id>`. With
  `async_remove`, the device is only marked and cleaned up during GC.
* **Device IDs** are allocated by containerd from its own bolt bucket
  (`getNextDeviceID`). A `create_snap` issued outside containerd with a
  self-chosen ID can collide with a later containerd allocation; containerd
  then marks its own device `Faulty`. Out-of-band devices are invisible to
  containerd GC and to `ResetPool`.
* **No point-in-time operation on an active snapshot** exists in the
  `Snapshotter` interface or as a devmapper extension, in v1.7, v2.3, or on
  `main`. A proposal to add `Snapshot`/`Restore` to snapshotters
  ([PR #13111](https://github.com/containerd/containerd/pull/13111)) was closed
  unmerged in August 2026.

### 5.2 Diff service

[`plugins/diff/walking/differ.go`](https://github.com/containerd/containerd/blob/main/plugins/diff/walking/differ.go)
mounts the lower and upper mounts on temporary paths and calls
`archive.WriteDiff`, which uses continuity's double-walk
([`fs/diff.go`](https://github.com/containerd/continuity/blob/main/fs/diff.go))
to emit an OCI tar layer with whiteouts. It works with block-device mounts, and
costs O(files) in both trees, but:

* it must never be pointed at the device a running VM has open; mounting a
  filesystem that a guest is using corrupts it;
* mounting a crash-consistent ext4 read-only still replays the journal
  ("write access will be enabled during recovery",
  [`fs/ext4/super.c`](https://github.com/torvalds/linux/blob/master/fs/ext4/super.c)),
  so it has to be run on a throwaway copy;
* the result is file-level and so unsuitable for memory restore (section 3).

### 5.3 blockfile snapshotter

Sources: [`blockfile.go`](https://github.com/containerd/containerd/blob/main/plugins/snapshots/blockfile/blockfile.go),
[`docs/snapshotters/blockfile.md`](https://github.com/containerd/containerd/blob/main/docs/snapshotters/blockfile.md).

* One raw filesystem image per snapshot at `<root>/snapshots/<id>`. Prepare
  copies the parent's file (or the configured `scratch_file`) with
  `copyFileWithSync`; a View with a parent reuses the parent's file read-only;
  Commit is metadata only. Each layer file holds the whole image, so there is
  no chain at mount time.
* The mount returned is `Type: <fs_type>` (default ext4, xfs also supported),
  `Source: <file>`, options `loop` plus `rw` or `ro`. Config keys are
  `root_path`, `scratch_file`, `recreate_scratch`, `fs_type`, `mount_options`.
* On Linux the copy is `io.Copy`, which Go implements with
  `copy_file_range(2)`. The kernel turns that into a reflink when both files
  are on the same filesystem and it supports `remap_file_range`
  ([`fs/read_write.c`](https://github.com/torvalds/linux/blob/master/fs/read_write.c),
  [`copy_file_range(2)`](https://man7.org/linux/man-pages/man2/copy_file_range.2.html)),
  so XFS with `reflink=1` and Btrfs get copy-on-write clones. An unmerged docs
  PR ([#13442](https://github.com/containerd/containerd/pull/13442)) states
  this; it is not in the merged docs. Sparse regions are not preserved
  ([#12956](https://github.com/containerd/containerd/issues/12956); two fixes
  closed unmerged).
* It first appears in the v1.7.x line after v1.7.0 and is present in v1.7.20;
  containerd 2.x has it under `plugins/snapshots/blockfile`. It is not marked
  experimental.
* There is no containerd operation to snapshot an active blockfile snapshot
  (PR #13111 would have added one).

### 5.4 erofs snapshotter

Sources: [`docs/snapshotters/erofs.md`](https://github.com/containerd/containerd/blob/main/docs/snapshotters/erofs.md),
[`erofs.go`](https://github.com/containerd/containerd/blob/main/plugins/snapshots/erofs/erofs.go),
[PR #12333](https://github.com/containerd/containerd/pull/12333),
[Kata erofs how-to](https://github.com/kata-containers/kata-containers/blob/main/docs/how-to/how-to-use-erofs-snapshotter-with-kata.md).

* Available from containerd v2.1, reworked in v2.2 to use the mount manager.
  Each layer is converted to a read-only `layer.erofs` blob at unpack time.
  Lower mounts are `erofs` with `ro,loop`, optionally merged through an
  `fsmeta.erofs` that lists the per-layer blobs as `device=` options.
* Without `default_size` the writable layer is a host directory under
  overlayfs. With `default_size` ("block mode") containerd emits a
  `mkfs/ext4` mount for `snapshots/<id>/rwlayer.img` and a `format/mkdir/
  overlay` mount whose `upperdir` and `workdir` live inside that image.
  Consumers must interpret these templated mount types or use containerd's
  mount manager.
* Kata consumes erofs layers as read-only virtio-blk devices plus an ext4
  writable device, assembling the overlay in the guest. Its docs say the
  multi-layer "fsmerged" form is QEMU only, and require containerd 2.2,
  erofs-utils 1.8.2, and a guest kernel of at least 5.4.

## 6. dm-thin facts

Sources: [`thin-provisioning.rst`](https://github.com/torvalds/linux/blob/master/Documentation/admin-guide/device-mapper/thin-provisioning.rst),
[`block/bdev.c`](https://github.com/torvalds/linux/blob/master/block/bdev.c),
[`thin_delta.txt`](https://github.com/jthornber/thin-provisioning-tools/blob/main/man8/thin_delta.txt),
[`delta_visitor.rs`](https://github.com/jthornber/thin-provisioning-tools/blob/main/src/thin/delta_visitor.rs).

* **Internal snapshots.** `dmsetup message <pool> 0 "create_snap <id>
  <origin>"`. "If the origin device that you wish to snapshot is active, you
  must suspend it before creating the snapshot to avoid corruption." The origin
  does not have to be closed: a VM can keep it open. The kernel does not
  enforce the suspend.
* **What suspend does.** A default `dmsetup suspend` freezes the block device
  (`bdev_freeze`); with no host filesystem on it, that is `sync_blockdev` plus
  waiting for in-flight bios. It typically takes milliseconds and blocks the
  VMM's I/O for that window, which is inside the VMM pause in any case.
* **Consistency.** The snapshot is crash-consistent at block level. Taken while
  the VMM is paused and drained, it matches the guest's memory image.
* **External snapshots.** A read-only origin outside the pool can back thin
  devices ("You must not write to the origin device"). On a target host, a
  read-only base could serve as the external origin for many clones.
* **`reserve_metadata_snap` / `release_metadata_snap`** reserve a copy of the
  mapping btree for userland tools. `thin_delta` "cannot be run on live
  metadata unless the --metadata-snap option is used". Its output is XML with
  `same`, `different`, `left_only`, and `right_only` ranges in *thin-block*
  units. Those offsets are the thin device's virtual addresses, independent of
  where the pool stores the data, so a delta is portable between pools (after
  converting between `data_block_size` values). The data itself is not
  included; it is read from the snapshot device. `thin_dump` gives physical
  mappings and is pool-specific.

## 7. Filesystem-level clones

Sources: [`ioctl_ficlone(2)`](https://man7.org/linux/man-pages/man2/ioctl_ficlone.2.html),
[`mkfs.xfs(8)`](https://man7.org/linux/man-pages/man8/mkfs.xfs.8.html),
[Btrfs reflink](https://btrfs.readthedocs.io/en/latest/Reflink.html),
[`zfs-snapshot(8)`](https://openzfs.github.io/openzfs-docs/man/master/8/zfs-snapshot.8.html),
[`zfs-send(8)`](https://openzfs.github.io/openzfs-docs/man/master/8/zfs-send.8.html),
[`unix.IoctlFileClone`](https://pkg.go.dev/golang.org/x/sys/unix#IoctlFileClone),
[`snapshot.rst`](https://docs.kernel.org/admin-guide/device-mapper/snapshot.html).

* `FICLONE` and `FICLONERANGE` share extents between two files on the same
  filesystem. "Clones are atomic with regards to concurrent writes, so no locks
  need to be taken to obtain a consistent cloned copy." Later writes to either
  file stay private. Errors are `EOPNOTSUPP` (no reflink support) and `EXDEV`
  (different filesystems).
* Cloning a disk file that a paused VMM has open gives a crash-consistent copy
  at that instant, in constant time relative to file size. XFS needs
  `mkfs.xfs -m reflink=1`; Btrfs supports it by default; ZFS offers dataset and
  zvol snapshots with `zfs send` for transfer.
* Go: `unix.IoctlFileClone(dst, src)` and `IoctlFileCloneRange`.
* LVM thin *is* dm-thin. dm-snapshot (non-thin) uses an origin plus a COW
  device that becomes unusable when full; Ignite used it.

## 8. Options

Each option answers the same questions: how a volume is created from an OCI
image; how it is captured while the VMM is paused; how the delta and base are
handled; how it is restored per VMM; what containerd still tracks; what changes
in flintlock; host requirements; risks.

### Option A: stay on devmapper

* **Provisioning.** Unchanged: `Prepare` on the image ChainID.
* **Capture.** While the VMM is paused: `dmsetup suspend` on the VM's thin
  device, `create_snap <new-id> <vm-id>`, `dmsetup resume`, activate the new
  device. Milliseconds. Works for both VMMs.
* **Delta and base.** `reserve_metadata_snap`, then `thin_delta` between the
  image's committed device and the new snapshot device gives the changed
  virtual block ranges; read them from the snapshot device. The base must be
  identical on the target (section 3.3): export the committed image device once
  per host as a block artifact, or ship the full volume.
* **Restore.** Pull the base artifact, write it into a thin device (or use it
  as an external origin), `create_snap` per clone, apply the delta ranges,
  then present the device. Firecracker: via the stable per-VM path. Cloud
  Hypervisor: rewrite `disks[].path` in `config.json`.
* **containerd tracking.** None for the capture. The new device's ID must not
  collide with containerd's allocator, so flintlock needs reserved IDs, a
  separate pool, or an upstream snapshotter operation. Captured devices and
  restored bases are outside containerd GC.
* **Flintlock changes.** New device-mapper code (dmsetup, thin-provisioning-
  tools), device ID and lifecycle management, base export. No change to
  provisioning.
* **Host requirements.** dm-thin pool (already required), thin-provisioning-
  tools on the host.
* **Risks.** Out-of-band device management is the main one; a containerd
  upgrade or `ResetPool` does not know about flintlock's devices.

### Option B: containerd blockfile on a reflink filesystem

* **Provisioning.** Switch the volume snapshotter to `blockfile`. `Prepare`
  reflinks the image's top-layer file into a per-VM raw file. The mount source
  is that file; both VMMs take a raw file as a virtio-blk drive.
* **Capture.** While paused, `FICLONE` the VM's file into a flintlock-owned
  capture file. Milliseconds, both VMMs.
* **Delta and base.** No native delta. Options: ship the whole sparse file; or
  reflink the base once (the image layer file) and compute changed extents by
  comparing capture and base (O(size) read, no pause); or use `FIEMAP` shared-
  extent heuristics (not guaranteed). The base portability problem is the same
  as Option A: the image layer file is built per host by untarring onto the
  scratch image.
* **Restore.** Reflink the base file per clone, apply the delta, present the
  file. Same VMM path handling as Option A.
* **containerd tracking.** The active volume is tracked; captures and clones
  made by flintlock are not.
* **Flintlock changes.** Small: a `blockfile` case in `convert.go` returning a
  `dev`-style mount whose source is the file; add it to
  `supportedSnapshotters`; add a volume-snapshotter flag (none exists); kernel
  and initrd stay on `native`. Provisioning scripts and docs change from thin
  pool to an XFS `reflink=1` or Btrfs directory and a scratch image.
* **Host requirements.** containerd daemon with blockfile (1.7.20 or later
  verified; not 1.7.0); XFS with reflink or Btrfs for constant-time copies,
  otherwise each Prepare is a full copy.
* **Risks.** Sparse files are not preserved by containerd's copy (#12956),
  which inflates disk use unless the host filesystem reflinks; the snapshotter
  is young; every layer is a full-size image file.

### Option C: read-only deterministic base plus a separate writable disk

* **Provisioning.** The image becomes a read-only block image built once and
  identified by digest (EROFS via the erofs snapshotter in block mode, or a
  flattened ext4 published as an artifact). Each VM gets that base as a
  read-only drive plus a fresh writable raw disk. The guest's init assembles
  an overlayfs (the pattern used by E2B, Ignite, and AWS Lambda; see
  section 9).
* **Capture.** Reflink or copy the writable disk while paused. Milliseconds
  with reflink. Both VMMs.
* **Delta and base.** The writable disk *is* the delta. The base is
  deterministic (EROFS) or content-addressed, so the target only needs the
  digest. Both the base and the writable disk are still mounted filesystems
  and must be byte-identical on restore, which this design gives naturally.
* **Restore.** Pull the base by digest, copy the writable disk per clone,
  present both drives in the original order.
* **containerd tracking.** The base is tracked; per-VM writable disks and
  captures are flintlock's.
* **Flintlock changes.** Largest: a second drive per volume, a writable-disk
  lifecycle, and, for erofs, containerd 2.2 and handling of the `mkfs/` and
  `format/` mount types. Guest images must ship an overlay-capable init.
  Existing images that expect a writable root would not work unchanged.
* **Host requirements.** Reflink filesystem for fast capture; guest kernel 5.4
  or later for EROFS; containerd 2.2 for the erofs snapshotter.
* **Risks.** Changes the guest image contract for every flintlock user;
  multi-layer EROFS over virtio-blk is documented for QEMU only, so images
  would need to be flattened to one blob.

### Option D: qcow2 overlay over a digest-named raw base (Cloud Hypervisor only)

* **Provisioning.** A raw base file per image plus a per-VM qcow2 overlay with
  a relative backing path, opened with `backing_files=on`.
* **Capture.** Copy or reflink the overlay while paused.
* **Delta and base.** The overlay is the delta and `qemu-img rebase -u`
  rewrites the backing reference if the base moves. Backing files are looked up
  "relative to the directory containing" the image
  ([`qemu-img`](https://gitlab.com/qemu-project/qemu/-/blob/master/docs/tools/qemu-img.rst)).
  The base must still be byte-identical on the target.
* **Restore.** Place the base and overlay, rewrite `config.json`.
* **VMMs.** Cloud Hypervisor only. Firecracker has no qcow2 support and
  vhost-user disables its snapshots.
* **Flintlock changes.** New file-based provisioning path, qcow2 overlay
  creation (header write or `qemu-img`), no containerd involvement for the
  per-VM disk.
* **Risks.** Splits the two providers onto different storage designs; Go qcow2
  writers are third-party
  ([go-qcow2reader](https://github.com/lima-vm/go-qcow2reader) is read-only).

## 9. Prior art

| Project | Root volume | Disk capture | Source |
| ------- | ----------- | ------------ | ------ |
| firecracker-containerd | devmapper thin snapshots; stub drives patched before the guest mounts them | none (control API has no snapshot RPC) | [snapshotter.md](https://github.com/firecracker-microvm/firecracker-containerd/blob/main/docs/snapshotter.md), [drive_handler.go](https://github.com/firecracker-microvm/firecracker-containerd/blob/main/runtime/drive_handler.go), [fccontrol.proto](https://github.com/firecracker-microvm/firecracker-containerd/blob/main/proto/service/fccontrol/fccontrol.proto) |
| Kata Containers | devmapper for Firecracker; erofs over virtio-blk plus ext4 writable device (fsmerged multi-layer is QEMU only) | none | [Firecracker how-to](https://github.com/kata-containers/kata-containers/blob/main/docs/how-to/how-to-use-kata-containers-with-firecracker.md), [erofs how-to](https://github.com/kata-containers/kata-containers/blob/main/docs/how-to/how-to-use-erofs-snapshotter-with-kata.md) |
| Ignite (archived) | read-only image file plus per-VM overlay file, joined with dm-snapshot over loop devices | none | [dmlegacy/snapshot.go](https://github.com/weaveworks/ignite/blob/main/pkg/dmlegacy/snapshot.go) |
| E2B | read-only template rootfs plus writable overlay; current code serves an NBD device combining base and a cache file | on pause: flush, swap in a fresh cache, reflink and export the frozen cache as a diff in the background | [overlayfs post](https://e2b.dev/blog/scaling-firecracker-using-overlayfs-to-save-disk-space), [orchestrator sandbox code](https://github.com/e2b-dev/infra/tree/main/packages/orchestrator/pkg/sandbox) |
| Fly.io Machines | LVM thin volumes with ext4 | migration is kill, clone, boot; dm-clone hydrates from the source over iSCSI | [storage post](https://fly.io/blog/persistent-storage-and-fast-remote-builds/), [migrations post](https://fly.io/blog/machine-migrations/) |
| Fly.io Sprites | ext4 on chunks in object storage with local SQLite metadata | checkpoint and restore "merely shuffle metadata around" | [design post](https://fly.io/blog/design-and-implementation/) |
| Tensorlake | per-VM sparse overlay over a shared base with dirty and zero bitmaps, inside Firecracker's block device | 29-129 ms per snapshot, content-addressed manifests | [blog](https://www.tensorlake.ai/blog/firecracker-disk-snapshots-o-changed-bytes) |
| AWS Lambda | image flattened deterministically to one ext4, split into 512 KiB content-named chunks; served to Firecracker via FUSE | writes go to a page-granular local overlay with a bitmap; base chunks stay shared | [ATC'23 paper](https://arxiv.org/abs/2305.13162) |
| CodeSandbox | copy-on-write for disks and memory (XFS reflink for memory files) | not verified beyond the post's summary | [blog](https://codesandbox.io/blog/how-we-clone-a-running-vm-in-2-seconds) |

Two patterns recur: a read-only, content-addressed base with a separate
writable layer (E2B, Ignite, Lambda, Tensorlake), and block-level deltas
captured while paused. None of the projects rebuild the disk from a file-level
diff for a memory restore.

## 10. Comparison

| | A. devmapper | B. blockfile + reflink | C. RO base + RW disk | D. qcow2 overlay |
| - | ------------ | ---------------------- | -------------------- | ---------------- |
| Capture while paused | suspend + `create_snap`, ms | `FICLONE`, ms | reflink RW disk, ms | reflink/copy overlay, ms |
| Delta against base | `thin_delta` virtual ranges | none native; compare against base | RW disk is the delta | overlay is the delta |
| Base portability | export per host, or ship full | export per host, or ship full | deterministic or by digest | export per host |
| Firecracker | yes | yes | yes (two drives) | **no** |
| Cloud Hypervisor | yes | yes | yes | yes |
| containerd tracks capture | no; device ID collision risk | no | no (base yes) | no |
| Provisioning change | none | small | large (guest init) | medium |
| Host requirements | thin pool, thin tools | XFS reflink / Btrfs, containerd with blockfile | reflink fs, containerd 2.2 (erofs) | CH only |
| Guest image change | none | none | overlay-capable init | none |

### Open questions for review

1. Which storage direction to adopt, or whether to support more than one
   behind the `ImageService` port.
2. How base block images are produced, distributed, cached, and garbage
   collected on hosts. The answer determines whether a delta is practical or
   the first version ships full volumes.
3. Whether to propose an upstream containerd operation for a point-in-time
   snapshot of an active snapshot (PR #13111 is a starting point). It would
   make Options A and B fully tracked by containerd.
4. The delta encoding: `thin_delta`-style virtual block ranges, a sparse raw
   image, or an existing format such as qcow2 as a container for the delta
   regardless of the VMM's on-disk format.
5. The stable per-VM drive path scheme for Firecracker, which has no drive
   override at load time.

## 11. References

Flintlock (commit `68533c7`):

* `core/ports/services.go`, `infrastructure/containerd/{image_service,convert,
  snapshot,lease,repo,config}.go`, `infrastructure/microvm/firecracker/config.go`,
  `infrastructure/microvm/cloudhypervisor/create.go`,
  `infrastructure/microvm/shared/imagefile.go`, `internal/inject/wire.go`,
  `pkg/defaults/defaults.go`, `hack/scripts/{devpool,direct_lvm}.sh`,
  `internal/provision/`.

Firecracker:

* <https://github.com/firecracker-microvm/firecracker/blob/main/src/firecracker/swagger/firecracker.yaml>
* <https://github.com/firecracker-microvm/firecracker/blob/main/docs/snapshotting/snapshot-support.md>
* <https://github.com/firecracker-microvm/firecracker/blob/main/docs/api_requests/patch-block.md>
* <https://github.com/firecracker-microvm/firecracker/blob/main/docs/api_requests/block-io-engine.md>
* <https://github.com/firecracker-microvm/firecracker/blob/main/docs/api_requests/block-vhost-user.md>
* <https://github.com/firecracker-microvm/firecracker/issues/4014>

Cloud Hypervisor:

* <https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/docs/snapshot_restore.md>
* <https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/docs/api.md>
* <https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/docs/hotplug.md>
* <https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/docs/disk_locking.md>
* <https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/vmm/src/api/openapi/cloud-hypervisor.yaml>
* <https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/vmm/src/config.rs>
* <https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/block/src/formats/qcow/backing.rs>
* <https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/vhost_user_block/src/lib.rs>

containerd:

* <https://github.com/containerd/containerd/blob/v1.7.35/snapshots/devmapper/snapshotter.go>
* <https://github.com/containerd/containerd/blob/v1.7.35/snapshots/devmapper/pool_device.go>
* <https://github.com/containerd/containerd/blob/v1.7.35/snapshots/devmapper/metadata.go>
* <https://github.com/containerd/containerd/blob/v1.7.35/snapshots/devmapper/config.go>
* <https://github.com/containerd/containerd/blob/v1.7.35/snapshots/storage/bolt.go>
* <https://github.com/containerd/containerd/issues/4234>
* <https://github.com/containerd/containerd/blob/main/plugins/diff/walking/differ.go>
* <https://github.com/containerd/containerd/blob/main/core/diff/apply/apply.go>
* <https://github.com/containerd/continuity/blob/main/fs/diff.go>
* <https://github.com/containerd/containerd/blob/main/plugins/snapshots/blockfile/blockfile.go>
* <https://github.com/containerd/containerd/blob/main/docs/snapshotters/blockfile.md>
* <https://github.com/containerd/containerd/pull/13442>
* <https://github.com/containerd/containerd/issues/12956>
* <https://github.com/containerd/containerd/pull/13111>
* <https://github.com/containerd/containerd/blob/main/docs/snapshotters/erofs.md>
* <https://github.com/containerd/containerd/blob/main/plugins/snapshots/erofs/erofs.go>
* <https://github.com/containerd/containerd/pull/12333>

Kernel, device-mapper, filesystems:

* <https://github.com/torvalds/linux/blob/master/Documentation/admin-guide/device-mapper/thin-provisioning.rst>
* <https://docs.kernel.org/admin-guide/device-mapper/snapshot.html>
* <https://github.com/torvalds/linux/blob/master/block/bdev.c>
* <https://github.com/torvalds/linux/blob/master/fs/read_write.c>
* <https://github.com/torvalds/linux/blob/master/fs/ext4/super.c>
* <https://github.com/jthornber/thin-provisioning-tools/blob/main/man8/thin_delta.txt>
* <https://github.com/jthornber/thin-provisioning-tools/blob/main/src/thin/delta_visitor.rs>
* <https://man7.org/linux/man-pages/man2/ioctl_ficlone.2.html>
* <https://man7.org/linux/man-pages/man2/copy_file_range.2.html>
* <https://man7.org/linux/man-pages/man8/mkfs.xfs.8.html>
* <https://btrfs.readthedocs.io/en/latest/Reflink.html>
* <https://openzfs.github.io/openzfs-docs/man/master/8/zfs-snapshot.8.html>
* <https://openzfs.github.io/openzfs-docs/man/master/8/zfs-send.8.html>
* <https://pkg.go.dev/golang.org/x/sys/unix#IoctlFileClone>
* <https://github.com/erofs/erofs-utils/blob/master/man/mkfs.erofs.1>

qcow2 and tooling:

* <https://gitlab.com/qemu-project/qemu/-/blob/master/docs/tools/qemu-img.rst>
* <https://gitlab.com/qemu-project/qemu/-/blob/master/docs/interop/live-block-operations.rst>
* <https://github.com/lima-vm/go-qcow2reader>

Prior art:

* <https://github.com/firecracker-microvm/firecracker-containerd/blob/main/docs/snapshotter.md>
* <https://github.com/firecracker-microvm/firecracker-containerd/blob/main/runtime/drive_handler.go>
* <https://github.com/firecracker-microvm/firecracker-containerd/blob/main/proto/service/fccontrol/fccontrol.proto>
* <https://github.com/kata-containers/kata-containers/blob/main/docs/how-to/how-to-use-kata-containers-with-firecracker.md>
* <https://github.com/kata-containers/kata-containers/blob/main/docs/how-to/how-to-use-erofs-snapshotter-with-kata.md>
* <https://github.com/weaveworks/ignite/blob/main/pkg/dmlegacy/snapshot.go>
* <https://e2b.dev/blog/scaling-firecracker-using-overlayfs-to-save-disk-space>
* <https://github.com/e2b-dev/infra/tree/main/packages/orchestrator/pkg/sandbox>
* <https://fly.io/blog/persistent-storage-and-fast-remote-builds/>
* <https://fly.io/blog/machine-migrations/>
* <https://fly.io/blog/design-and-implementation/>
* <https://www.tensorlake.ai/blog/firecracker-disk-snapshots-o-changed-bytes>
* <https://arxiv.org/abs/2305.13162>
* <https://codesandbox.io/blog/how-we-clone-a-running-vm-in-2-seconds>
