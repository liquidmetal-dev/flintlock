# 0204. Option C: read-only deterministic base plus a per-VM writable disk

* Status: proposed design candidate
* Date: 2026-09-25
* Authors: @richardcase
* Issue: [#204](https://github.com/liquidmetal-dev/flintlock/issues/204)
* Companions: [requirements](0204-snapshot-restore-requirements.md),
  [options survey](0204-root-volume-options.md),
  [Option A](0204-option-a-devmapper.md), [Option B](0204-option-b-blockfile.md)

## 1. Summary

Each volume becomes two drives. The first is a read-only base block image
built deterministically from the OCI image (EROFS, or ext4 with pinned
inputs), identified by the digest of its bytes and identical on every host.
The second is a small per-VM writable ext4 disk. The guest's init mounts the
base, mounts the writable disk, and assembles an overlayfs root from the two.
Provisioning a VM is a reflink of a pre-formatted empty writable disk. A
snapshot captures the writable disk with one `FICLONE`; that file is the whole
delta. A clone pulls the base by digest, once per host for all snapshots of
that image, copies the writable disk, and boots.

This is the design used by firecracker-containerd's image builder and, in its
early form, by E2B. It has the smallest and most portable delta and the
smallest per-clone work of the three options, and the largest contract change:
guest images must carry an overlay-capable init.

## 2. Requirements mapping

| Requirement | How this design meets it |
| ----------- | ------------------------ |
| SNAP-VOL-001 | the base is byte-identical by construction (deterministic build, digest verified); the writable disk is FICLONE'd byte for byte. |
| SNAP-VOL-003 | FICLONE inside the pause. |
| SNAP-VOL-004/005 | one ioctl per writable disk, O(extents); the writable disk holds only guest changes so its dirty cache is small. |
| SNAP-VOL-006 | pull base by digest, reflink the shipped writable disk, present both. |
| SNAP-VOL-007 | per-VM `volumes/<id>.base` and `volumes/<id>.rw` links; relative paths for Firecracker, `config.json` rewrite for CH. |
| SNAP-VOL-008 | the source's containerd snapshot (base) is untouched; the writable disk is flintlock's. |
| SNAP-VOL-009 | delta = the writable disk as a sparse block stream (block delta v1 with no base, or against an empty template). |
| SNAP-VOL-010 | read-only volumes are just a base drive; digest recorded. |
| SNAP-VOL-011 | base by digest from any host or registry that has it; verified. |
| SNAP-VOL-012 | ledger of writable disks and bases as in Option B. |
| SNAP-CRT-005/007 | capture inside pause; compress and push afterwards. |
| SNAP-CLN-001/002 | each clone has its own writable disk; bases are shared read-only. |
| SNAP-META-001 | descriptor records base digests and the writable disk size per volume. |

## 3. Host and image contract

* Host filesystem with reflink (XFS `reflink=1` or Btrfs) for the writable
  disks and bases, as in Option B. Without it, capture and provisioning are
  O(writable disk allocated size), which is still small.
* Base construction, one of:
  * **EROFS via containerd >= 2.2 erofs snapshotter in block mode.** The
    differ converts each layer with `mkfs.erofs --tar=f --aufs
    -Enoinline_data -U <sha1 uuid of layer digest> [mkfs_options]`
    ([`mount.go`](https://github.com/containerd/containerd/blob/main/internal/erofsutils/mount.go),
    [`differ.go`](https://github.com/containerd/containerd/blob/main/plugins/diff/erofs/differ.go));
    with `mkfs_options = ["-T0", "--mkfs-time", "--sort=none"]` the layer
    blobs are reproducible for a fixed erofs-utils version
    ([`erofs.md`](https://github.com/containerd/containerd/blob/main/docs/snapshotters/erofs.md)).
    The snapshotter returns templated mounts that flintlock must interpret
    itself (section 5.2).
  * **Flattened image built by flintlock** (containerd 1.7 compatible): merge
    the image's layers into one tree (containerd `native` snapshotter `View`
    of the image gives a directory), then `mkfs.erofs -T<epoch> -U <uuid
    derived from the OCI image digest> --sort=path -zlz4 ...` or
    `mke2fs -d <dir> -U <uuid> -E hash_seed=<uuid>,lazy_itable_init=0,
    lazy_journal_init=0` with `SOURCE_DATE_EPOCH` set
    ([`mkfs.erofs(1)`](https://git.kernel.org/pub/scm/linux/kernel/git/xiang/erofs-utils.git/tree/man/mkfs.erofs.1),
    [`mke2fs(8)`](https://git.kernel.org/pub/scm/fs/ext2/e2fsprogs.git/tree/misc/mke2fs.8.in),
    [`create_inode.c`](https://git.kernel.org/pub/scm/fs/ext2/e2fsprogs.git/tree/misc/create_inode.c)
    uses `scandir` with `alphasort`).
  * **An external image pipeline** publishes the base artifact alongside the
    OCI image; flintlock only consumes.
  Reproducibility depends on pinning the tool version: erofs-utils'
  ChangeLog records reproducibility fixes in 1.8.3, 1.8.7 and 1.9.3
  ([ChangeLog](https://git.kernel.org/pub/scm/linux/kernel/git/xiang/erofs-utils.git/tree/ChangeLog)).
* Guest kernel: `CONFIG_EROFS_FS` (plus the compression option used) and
  `CONFIG_OVERLAY_FS`; ext4 for the writable disk
  ([`fs/erofs/Kconfig`](https://github.com/torvalds/linux/blob/master/fs/erofs/Kconfig)).
  Overlay upper needs xattrs and d_type, which ext4 provides
  ([overlayfs.rst](https://github.com/torvalds/linux/blob/master/Documentation/filesystems/overlayfs.rst)).
* Guest image: an init that assembles the overlay. Reference:
  firecracker-containerd's `overlay-init`, which mounts `/dev/$overlay_root`
  as ext4 on `/overlay`, creates `root` and `work`, mounts
  `overlay lowerdir=/ upperdir=/overlay/root workdir=/overlay/work` on
  `/mnt`, `pivot_root /mnt /mnt/rom`, and execs the real init
  ([overlay-init](https://github.com/firecracker-microvm/firecracker-containerd/blob/main/tools/image-builder/files_debootstrap/sbin/overlay-init)).
  The kernel passes unknown `key=value` boot parameters to init's environment
  ([kernel-parameters.rst](https://github.com/torvalds/linux/blob/master/Documentation/admin-guide/kernel-parameters.rst)),
  so flintlock adds `init=/sbin/overlay-init overlay_root=vdb rootfstype=erofs
  ro` to the kernel command line. Images without such an init cannot be used
  with this volume mode; the spec declares the mode per volume (section 5.4).

## 4. On-host layout

```text
<StateRootDir>/
├── vm/<ns>/<name>/<uid>/volumes/
│   ├── root.base -> <StateRootDir>/bases/<digest>.img     # read-only drive
│   └── root.rw                                            # writable ext4 disk (reflink of template)
├── bases/<digest>.img                                     # deterministic base images, 0400
├── templates/rw-<size>.img                                # pre-formatted empty ext4 disks
├── captures/<snapshot-id>/<volume-id>.rw                  # FICLONE captures
├── packages/<snapshot-id>/
└── volumes.db
```

## 5. Flintlock design

Port as [Option B section 5.1](0204-option-b-blockfile.md#51-new-port-volumeservice),
implemented in `infrastructure/volume/overlay`. `Create` returns two mounts
per volume; `models.VolumeStatus` grows a second mount (`Base`, `RW`), and the
providers emit two drives in a fixed order per volume.

### 5.1 Base production

`Base(image)`: if `bases/<digest>.img` for the image's OCI digest is present
(the ledger maps OCI digest → base digest), return it. Otherwise build it with
the configured builder (erofs snapshotter output, flintlock flatten, or pull
from the artifact repository configured for bases), verify reproducibility on
first use by building twice when a `verify-reproducible` flag is set, record,
and push the artifact. The base digest is the sha256 of the image file.

### 5.2 Consuming the erofs snapshotter (containerd >= 2.2)

In block mode the snapshotter returns `mkfs/ext4` (for `rwlayer.img`), one
`erofs` mount per layer blob or an `fsmeta.erofs` with `device=` options, and a
`format/mkdir/overlay` mount
([`erofs.go`](https://github.com/containerd/containerd/blob/main/plugins/snapshots/erofs/erofs.go),
[`mounts.md`](https://github.com/containerd/containerd/blob/main/docs/mounts.md)).
The gRPC mount manager cannot be asked to leave these unactivated
([`mounts.proto`](https://github.com/containerd/containerd/blob/main/api/services/mounts/v1/mounts.proto)),
so flintlock parses the list: it takes the erofs sources as base drives (one
flattened blob preferred; Kata's multi-layer VMDK trick is QEMU-only), creates
the writable disk itself from the `mkfs/ext4` template values, and translates
the `format/mkdir/overlay` template into the kernel parameters for the guest
init. Multi-blob images need one drive per blob and `device=` mounts in the
guest, which the reference `overlay-init` does not do; the design therefore
uses a single flattened base per image.

### 5.3 Writable disk

`templates/rw-<size>.img` is created once per size by `truncate` +
`mkfs.ext4 -E lazy_itable_init=0,lazy_journal_init=0 -U <fixed>` (the way
containerd's mkfs transformer does,
[`mkfs.go`](https://github.com/containerd/containerd/blob/main/core/mount/manager/mkfs.go));
`Create` reflinks it to `volumes/<id>.rw`. Size comes from `Volume.Size`
(carried by the API but unused today,
`infrastructure/grpc/convert.go:145`), default from config. Growth: Cloud
Hypervisor `vm.resize-disk`; Firecracker `PATCH /drives` only when the guest
has the device unmounted; the guest must run `resize2fs`. Guest TRIM with
Firecracker `discard: true` or Cloud Hypervisor `sparse=on` punches holes and
keeps captures small.

### 5.4 Spec and API

`Volume` gains `mode: full | overlay` (proto `VolumeMode`), default `full` for
compatibility; `overlay` requires a provider capability
`OverlayVolumeCapability` and an image that declares an overlay init (an image
label `dev.liquidmetal.flintlock/overlay-init=/sbin/overlay-init`, or a spec
field). The cloud-init disk mount step (`core/steps/cloudinit/disk_mount.go`)
assumes `vda` root then `vdb...`; with two drives per volume the device
lettering shifts, so the step derives letters from the emitted drive order.

### 5.5 Providers and provisioning

Firecracker: two drives per volume, base `is_read_only: true`, kernel args
extended, `cmd.Dir` and relative paths as in Option B. Cloud Hypervisor: two
`--disk` entries, `readonly=on` for the base, `--cmdline` extended.
Provisioning:
reflink directory as Option B, plus erofs-utils (and containerd 2.2 if the
snapshotter route is chosen) in `AptPackages`.

## 6. Lifecycle flows

**First VM start, image not on host.** (1) pull image. (2) build or pull the
base (O(image), once per host and image). (3) reflink template → writable disk
(O(extents), tiny). (4) boot; init assembles overlay (two mounts and a
`pivot_root`). No copy-on-write on reads.

**First VM start, image on host.** (3) and (4).

**Snapshot, resume.** pause → VMM snapshot → `fsync` writable disk (CH) →
`FICLONE` writable disk → resume → background: stream the capture as a sparse
image (SEEK_DATA/SEEK_HOLE), zstd, package (base referenced by digest, never
shipped), push.

**Snapshot, stop.** Hold the pause through packaging; stop at the stop point.

**Clone restore, base not cached.** pin and check → pull base by digest (any
snapshot of this image on any host shares it) → reflink the shipped writable
disk into the clone's `volumes/<id>.rw` → link base → VMM load → resume.

**Clone restore, base cached.** Reflink and link only.

**Delete.** Writable disks and links are unlinked by the volume release step;
bases are refcounted in the ledger and removed past retention; containerd's
image snapshot (if the erofs route is used) dies with the lease.

**Daemon restart.** As Option B.

## 7. Failure modes

| Step | Failure | State left | Handling |
| ---- | ------- | ---------- | -------- |
| base build | non-reproducible output (tool version drift, unpinned flags) | digest differs from other hosts; clones from other hosts fail verification | pin erofs-utils version in provisioning; optional double-build check; prefer pulling a published base over building |
| guest init | image lacks overlay init or kernel lacks EROFS | guest fails to boot | mode is explicit in the spec; image label checked at create; documented |
| writable disk full | guest sees ENOSPC | guest-visible | size from spec; resize path documented |
| FICLONE | as Option B | | |
| base file modified | clones diverge | never: 0400, verified on pull and on a periodic check | |
| device lettering | cloud-init mounts wrong device | guest config error | derive from drive order |
| restore with base missing in every configured repository | cannot proceed | none | fail before touching the VM (SNAP-RST-006 ordering) |

## 8. Base block image

* **What.** A deterministic filesystem image of the flattened OCI image.
  Identified by the sha256 of its bytes; the ledger and the package record
  both the OCI image digest and the base digest, and the qualified reference
  of the base artifact.
* **Count.** One per OCI image (for a given builder version and options),
  regardless of host: every snapshot of every VM from that image on any host
  references the same base. This is the property Options A and B lack.
* **Distribution.** Base artifacts live next to the images in the registry
  (`<repo>/bases:<digest>` or as referrers of the image manifest); hosts
  pull once and cache under lease; GC past retention.
* **Verification.** sha256 on pull; optional reproducibility check by
  rebuilding locally when the tools are present.

## 9. Delta format

The writable disk itself, streamed as block delta v1 with no base: data
records for SEEK_DATA ranges, holes implied. Size is O(allocated bytes of the
writable disk), which is the guest's changes plus ext4 metadata for the
writable filesystem. No comparison step is needed.

## 10. Experiments

Btrfs reflink results (shared with Option B, section 10) cover FICLONE
semantics, copy_file_range reflinking and sparse enumeration for the writable
disk.

C-1, `mkfs.erofs` (erofs-utils 1.9.4) run twice on the same source tree
(files with fixed mtimes), script `exp-c-erofs.sh`:

```text
### pinned: -T0 --mkfs-time -U <fixed> --sort=none -zlz4 (twice)
e792f4582272942721caf2fa8e63c8dd55f78cef52d2c2604236ae06d48711e0  a1.erofs
e792f4582272942721caf2fa8e63c8dd55f78cef52d2c2604236ae06d48711e0  a2.erofs
### without -U (twice)
f1fad8969a51f24f64ed23c747d2ac37989a6b7f9c9303dce86b49d15a9701c9  b1.erofs
f9c360c97a9216d47061e010eccdda9b66796005edb7fdd092c6f42441bddc87  b2.erofs
### with -U but without -T (twice)
76535edbf6534bc59d87fb3cd2ca1f843cc2a9e4943f35b7e8576b2be6c1f62d  c1.erofs
76535edbf6534bc59d87fb3cd2ca1f843cc2a9e4943f35b7e8576b2be6c1f62d  c2.erofs
```

The UUID is the decisive input: two builds differ without `-U` and match
with it. Without `-T` the output still matched because the source mtimes were
already fixed; a tree unpacked from OCI layers carries the layers' mtimes, so
`-T` matters when those are not stable. Note that `-q` is not accepted by
erofs-utils 1.9.4.

C-2, EROFS lower (loop, ro) plus ext4 upper (loop, rw) under overlayfs on the
host, the sequence a guest init performs:

```text
rw/upper/etc/motd          -rw-r--r-- 8 bytes      # modified file copied up
rw/upper/newfile           -rw-r--r-- 4 bytes      # new file
rw/upper/usr/bin/blob      c--------- 0, 0         # whiteout for the deleted file
```

Writes land in the ext4 upper as copies, new files and 0/0 whiteout devices,
and the EROFS lower is untouched. The writable disk therefore carries exactly
the guest's changes.

## 11. Effort and migration

* Code: the shared port and steps; `infrastructure/volume/overlay` (base
  builder or erofs mount parser, template disks, FICLONE, sparse stream);
  two-drive status and provider output; kernel args; `VolumeMode` in the API
  and validation; cloud-init lettering; capability. Runtime dependency on
  erofs-utils or e2fsprogs for building bases (none if bases are pulled).
* Guest images: must add an overlay init and EROFS support; existing
  flintlock images (for example the CAPMVM Ubuntu images) need a rebuild.
  `mode: full` keeps old images working without snapshot support.
* Hosts: reflink filesystem as Option B; optionally containerd 2.2.
* Upstream: none required; containerd 2.2's erofs snapshotter is optional.

## 12. Risks and open questions

* Guest contract change is the main cost: an init in every image and a kernel
  with EROFS. This is the same requirement Kata and firecracker-containerd
  place on their images.
* Reproducibility across erofs-utils versions is not guaranteed; a fleet must
  pin the builder or publish bases centrally.
* Two drives per volume changes device naming inside the guest and the
  cloud-init mount step.
* Writable disk sizing is fixed at create; growth needs guest cooperation.
* Whether to depend on containerd 2.2's erofs snapshotter or build flat bases
  in flintlock is open; the flat build works with containerd 1.7.

## 13. References

* containerd erofs: <https://github.com/containerd/containerd/blob/main/docs/snapshotters/erofs.md>,
  <https://github.com/containerd/containerd/blob/main/plugins/snapshots/erofs/erofs.go>,
  <https://github.com/containerd/containerd/blob/main/internal/erofsutils/mount.go>,
  <https://github.com/containerd/containerd/blob/main/plugins/diff/erofs/differ.go>,
  <https://github.com/containerd/containerd/blob/main/docs/mounts.md>,
  <https://github.com/containerd/containerd/blob/main/core/mount/manager/mkfs.go>,
  <https://github.com/containerd/containerd/blob/main/core/mount/manager.go>,
  <https://github.com/containerd/containerd/blob/main/api/services/mounts/v1/mounts.proto>,
  <https://github.com/containerd/containerd/blob/main/core/runtime/v2/task_mounts.go>
* erofs-utils: <https://git.kernel.org/pub/scm/linux/kernel/git/xiang/erofs-utils.git/tree/man/mkfs.erofs.1>,
  <https://git.kernel.org/pub/scm/linux/kernel/git/xiang/erofs-utils.git/tree/mkfs/main.c>,
  <https://git.kernel.org/pub/scm/linux/kernel/git/xiang/erofs-utils.git/tree/lib/inode.c>,
  <https://git.kernel.org/pub/scm/linux/kernel/git/xiang/erofs-utils.git/tree/ChangeLog>
* e2fsprogs: <https://git.kernel.org/pub/scm/fs/ext2/e2fsprogs.git/tree/misc/create_inode.c>,
  <https://git.kernel.org/pub/scm/fs/ext2/e2fsprogs.git/tree/misc/mke2fs.8.in>,
  <https://git.kernel.org/pub/scm/fs/ext2/e2fsprogs.git/tree/doc/RelNotes/v1.47.1.txt>
* Kernel: <https://github.com/torvalds/linux/blob/master/Documentation/filesystems/erofs.rst>,
  <https://github.com/torvalds/linux/blob/master/Documentation/filesystems/overlayfs.rst>,
  <https://github.com/torvalds/linux/blob/master/Documentation/admin-guide/kernel-parameters.rst>,
  <https://github.com/torvalds/linux/blob/master/fs/erofs/Kconfig>
* Guest init: <https://github.com/firecracker-microvm/firecracker-containerd/blob/main/tools/image-builder/files_debootstrap/sbin/overlay-init>;
  E2B 2023 post <https://e2b.dev/blog/scaling-firecracker-using-overlayfs-to-save-disk-space>
* Kata: <https://github.com/kata-containers/kata-containers/blob/main/docs/how-to/how-to-use-erofs-snapshotter-with-kata.md>,
  <https://github.com/kata-containers/kata-containers/blob/main/src/runtime-rs/crates/resource/src/rootfs/erofs_rootfs.rs>
* VMMs: <https://github.com/firecracker-microvm/firecracker/blob/main/docs/api_requests/block-discard.md>,
  <https://github.com/firecracker-microvm/firecracker/blob/main/docs/api_requests/patch-block.md>,
  <https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/docs/api.md>
* Flintlock: `core/steps/cloudinit/disk_mount.go`,
  `infrastructure/grpc/convert.go`, `api/types/microvm.proto`,
  `core/models/{volumes,capability}.go`
