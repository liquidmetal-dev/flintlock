# 0204. Option B: containerd blockfile snapshotter on a reflink filesystem

* Status: considered, may be revisited (see [decision](0204-root-volume-decision.md))
* Date: 2026-09-25
* Authors: @richardcase
* Issue: [#204](https://github.com/liquidmetal-dev/flintlock/issues/204)
* Companions: [requirements](0204-snapshot-restore-requirements.md),
  [options survey](0204-root-volume-options.md),
  [Option A](0204-option-a-devmapper.md), [Option C](0204-option-c-ro-base-rw-disk.md),
  [decision](0204-root-volume-decision.md)

## 1. Summary

Each volume is a raw ext4 image file produced by containerd's `blockfile`
snapshotter on an XFS (`reflink=1`) or Btrfs directory. `Prepare` reflinks the
image's top-layer file into a per-VM file, so provisioning is a metadata
operation and blocks are shared until written. The VMM opens the file directly
as a virtio-blk drive. A snapshot captures the volume with one `FICLONE` while
the VMM is paused; the capture is a byte-identical, independent copy. The delta
is computed afterwards by comparing the capture's physical extent map with a
reflinked copy of the base (the image's top-layer file), and shipped as block
ranges. A clone reflinks the base on the target host, writes the ranges, and
starts the VMM against the result.

## 2. Requirements mapping

| Requirement | How this design meets it |
| ----------- | ------------------------ |
| SNAP-VOL-001 (byte-identical, block-level) | FICLONE shares extents byte for byte; the delta is at 4 KiB block granularity against the same base bytes on both hosts. |
| SNAP-VOL-003 (paused until captured) | FICLONE runs inside the pause; it returns only after the dirty page cache is written back and the extents are shared. |
| SNAP-VOL-004/005 (shortest pause, pause limit) | One ioctl per volume; cost is O(extents) plus writeback of dirty cache (section 10). Writeback is bounded because Firecracker fsyncs on snapshot create; flintlock fsyncs for Cloud Hypervisor first. |
| SNAP-VOL-006 (recreate from base + delta) | Reflink the cached base file, `pwrite` ranges, fsync. |
| SNAP-VOL-007 (unique per-VM paths) | Volume files live under the VM's state directory; Firecracker opens `volumes/<id>` relative to a per-VM working directory; Cloud Hypervisor gets the absolute path rewritten in `config.json`. |
| SNAP-VOL-008 (source stays tracked) | The active containerd snapshot is untouched; the capture is a separate flintlock-owned file. |
| SNAP-VOL-009 (delta + base digest, or full image) | Block delta v1 stream against the base digest; a full sparse image is the same stream with no base. |
| SNAP-VOL-010 (read-only volumes) | Read-only volumes are `View` snapshots of the same base file; the base digest is recorded and the target reflinks it read-only. |
| SNAP-VOL-011 (obtain and verify base) | Base artifact keyed by the digest of the top-layer file bytes; pulled once per host, verified, cached under a lease. |
| SNAP-VOL-012 (track out-of-band state) | Captures, bases and clone volumes are recorded in flintlock's volume ledger and removed by the delete plans and a GC sweep. |
| SNAP-CRT-005/007 (order, stop point) | Capture inside pause; delta, compress and push afterwards; for stop, the VM stays paused until the package is complete locally. |
| SNAP-RST-010 (memory file retained) | Unchanged from the requirements: the memory file sits in the package store under the clone's lease. |
| SNAP-CLN-001/002 (many clones) | Each clone is an independent reflink of the base under its own VM directory. |
| SNAP-API-005/006 (delete) | Deleting the snapshot record removes the capture and package files not referenced by a clone lease. |
| SNAP-OPS-004 (restart) | A capture file without a matching record is swept; a paused VM is resumed. |

## 3. Host and image contract

* Host filesystem for the volume root: XFS with `reflink=1` (default since
  xfsprogs 5.1.0,
  [`mkfs.xfs(8)`](https://man7.org/linux/man-pages/man8/mkfs.xfs.8.html),
  [xfsprogs CHANGES](https://git.kernel.org/pub/scm/fs/xfs/xfsprogs-dev.git/tree/doc/CHANGES))
  or Btrfs, which always supports reflink
  ([`fs/btrfs/file.c`](https://github.com/torvalds/linux/blob/master/fs/btrfs/file.c)).
  Any other filesystem makes every `Prepare` an O(size) copy.
* containerd with the `blockfile` snapshotter: present in the 1.7 line
  (verified at 1.7.20, absent at 1.7.0) and in 2.x. The 1.7 plugin exposes
  `root_path`, `scratch_file`, `fs_type`, `mount_options`; 2.x adds
  `recreate_scratch`
  ([1.7 plugin](https://github.com/containerd/containerd/blob/release/1.7/snapshots/blockfile/plugin/plugin.go),
  [main plugin](https://github.com/containerd/containerd/blob/main/plugins/snapshots/blockfile/plugin/plugin.go)).
* A pre-formatted scratch file: containerd never runs `mkfs`; the operator
  creates `scratch` (`dd` + `mkfs.ext4`) and its size is the fixed size of
  every volume filesystem
  ([`blockfile.md`](https://github.com/containerd/containerd/blob/main/docs/snapshotters/blockfile.md)).
* Kernel >= 5.3 for `copy_file_range` reflinking through Go's `io.Copy`
  ([`copy_file_range_linux.go`](https://github.com/golang/go/blob/master/src/internal/poll/copy_file_range_linux.go)).
* No change to guest images: the guest still sees one writable root block
  device with the image's filesystem.
* Kernel and initrd stay on the `native` snapshotter.

## 4. On-host layout

```text
<StateRootDir>/
├── vm/<ns>/<name>/<uid>/                 # per-VM state dir (exists today)
│   ├── firecracker.cfg | cloudhypervisor.*  # as today
│   └── volumes/
│       ├── root  -> <blockfile root>/snapshots/<id>   # symlink to the active snapshot file
│       └── data1 -> ...
├── captures/<snapshot-id>/<volume-id>.img   # FICLONE captures (0600), removed after packaging
├── bases/<base-digest>.img                  # cached base block images (reflink source for clones)
├── packages/<snapshot-id>/                  # OCI image layout (SNAP-PKG-011)
└── volumes.db                               # ledger: bolt file, see section 5
<blockfile root_path>/                       # containerd-owned, on XFS reflink or Btrfs
├── scratch
├── snapshots/<id>                           # one raw ext4 image per snapshot
└── metadata.db
```

Everything under `captures/`, `bases/` and `packages/` is on the same reflink
filesystem as the blockfile root, so clones and captures share extents.
Directories are owned by the flintlockd user with mode 0700; files 0600
(SNAP-SEC-001).

## 5. Flintlock design

### 5.1 New port: `VolumeService`

`ImageService` stays for kernel and initrd. Volumes move behind a new port so
the three options are interchangeable behind one interface:

```go
// core/ports/services.go
type VolumeService interface {
    // Create provisions a writable (or read-only) volume for a VM from an
    // image and returns the host mount to hand to the VMM.
    Create(ctx context.Context, in VolumeCreateInput) (models.Mount, error)
    // Capture takes a point-in-time copy of a VM's volume. The VMM must be
    // paused. Returns a handle flintlock owns until Release.
    Capture(ctx context.Context, in VolumeCaptureInput) (VolumeCapture, error)
    // Base returns the base block image for a volume (exporting it from the
    // committed image snapshot on first use) and its digest.
    Base(ctx context.Context, in VolumeBaseInput) (BaseImage, error)
    // Delta streams the block ranges of a capture that differ from its base.
    Delta(ctx context.Context, cap VolumeCapture, base BaseImage) (io.ReadCloser, error)
    // Restore creates a VM volume from a base plus a delta stream.
    Restore(ctx context.Context, in VolumeRestoreInput) (models.Mount, error)
    // Release removes a capture or a restored volume.
    Release(ctx context.Context, ref VolumeRef) error
}
```

Inputs carry `VMID`, `VolumeID`, `Image` (OCI ref), `ReadOnly`, `BaseDigest`,
`Delta io.Reader`, and the per-VM `Dir`. `BaseImage{Digest, Path, Size}`.

### 5.2 containerd adapter (`infrastructure/containerd`)

* `config.go`: add `blockfile` to `supportedSnapshotters`; new
  `Config.SnapshotterVolume` is wired from a new flag
  `--containerd-volume-ss` (`internal/command/flags/flags.go`,
  `internal/config/config.go`), replacing the hardcoded default in
  `internal/inject/wire.go:73-80`.
* `convert.go`: a `blockfile` case returning `models.Mount{Type:
  MountTypeDev, Source: mount.Source}`. The source is the file path; both
  VMMs accept a regular file as a drive. The `loop` option is ignored: no
  loop device is created because flintlock never mounts the file.
* `image_service.go`: read-only volumes use `View` instead of `Prepare`
  (today every volume is `Prepare`); with blockfile a `View` points at the
  parent's file without a copy.
* New `infrastructure/volume/blockfile/service.go` implementing
  `VolumeService` on top of `ImageService` for `Create`, plus FICLONE,
  FIEMAP and range I/O for the rest (Go: `unix.IoctlFileClone`, FIEMAP via
  `unix.IoctlFiemap` or a raw ioctl, `unix.Pwrite`).

### 5.3 Ledger

`volumes.db` (bolt, same library containerd uses) with buckets `captures`,
`bases`, `clones`, keyed by snapshot id or base digest, each holding the file
path, size, digest, the VM or snapshot that owns it, and a lease name. The
controllers and the API server run separate port sets today (two
`InitializePorts` calls in `internal/command/run/run.go`), so the ledger opens
the file with a flock rather than an in-process mutex.

### 5.4 Steps and plans

* `core/steps/runtime/volume_mount.go` calls `VolumeService.Create` instead of
  `ImageService.PullAndMount`; `ShouldDo` also checks the per-VM symlink.
* New `core/steps/runtime/volume_release.go` in `microvm_delete.go`, before
  `NewRepoRelease`, removing `volumes/` links and any clone volume file the
  ledger attributes to the VM. The containerd snapshot itself still dies with
  the lease.
* Snapshot plan (new `core/plans/snapshot_create.go`): quiesce (optional),
  pause, VMM snapshot, `VolumeService.Capture` per mounted volume, resume or
  hold (stop point), then `Base`, `Delta`, package, push, release captures.
* Restore branch in `microvm_create_update.go`: when the spec carries a
  snapshot source, `VolumeService.Restore` replaces `Create` per volume, and
  the provider is asked to load rather than boot.

### 5.5 Providers

* Firecracker (`infrastructure/microvm/firecracker/create.go`): set
  `cmd.Dir` to the VM state directory and write drives as
  `path_on_host: "volumes/<id>"`. Firecracker opens the path as given
  (`open_file` in
  [`device.rs`](https://github.com/firecracker-microvm/firecracker/blob/main/src/vmm/src/devices/virtio/block/virtio/device.rs)),
  so the same string resolves to each clone's own file at load time
  (SNAP-VOL-007). The API socket replaces `--no-api` (SNAP-PRV-001).
* Cloud Hypervisor: `--disk path=<absolute volumes/<id>>` as today; on restore
  rewrite `disks[].path` in `config.json`. Add `readonly=on` for read-only
  volumes (currently ignored).
* Both: set `discard` (Firecracker >= 1.17.0,
  [`block-discard.md`](https://github.com/firecracker-microvm/firecracker/blob/main/docs/api_requests/block-discard.md))
  and keep `sparse=on` (Cloud Hypervisor default) so guest TRIM punches holes
  in the file and shrinks captures.

### 5.6 Provisioning

`internal/provision`: a `storage=blockfile` mode in `setupStorage` that
formats or validates a reflink-capable directory (`mkfs.xfs -m reflink=1` on a
dedicated device, or a check for `reflink=1` / Btrfs), creates the scratch
file, and emits a `[plugins."io.containerd.snapshotter.v1.blockfile"]` block
from `BuildContainerdConfig`. `AptPackages` gains `xfsprogs`. The e2e runner
(`test/e2e/utils/runner.go`) gains the same switch; the integration test
constant `testSnapshotter` becomes an environment variable.

### 5.7 Untouched

Network, vsock, metadata service, cloud-init disk (still a per-VM file),
kernel and initrd mounts, the gRPC API for existing VM creation.

## 6. Lifecycle flows

**First VM start, image not on host.** (1) flintlock pulls the image
(containerd, network, O(image)). (2) containerd unpacks: copy scratch, loop
mount, untar layer, unmount, Commit, per layer; on a reflink filesystem each
copy is a reflink and the untar writes only new data. (3) `Prepare` for the VM:
one `copy_file_range` of the top-layer file, a reflink (O(extents), 1 ms for
256 MiB in experiment B/C-2). (4) flintlock links `volumes/root` to the file.
(5) VMM boots; the guest's first write to each shared block allocates a new
block in the filesystem.

**First VM start, image on host.** Steps 3 to 5 only.

**Snapshot, resume.** (1) optional quiesce. (2) pause. (3) VMM snapshot
(Firecracker fsyncs the drive files; for Cloud Hypervisor flintlock calls
`fsync` on each volume file). (4) `FICLONE` each volume into
`captures/<snap>/<vol>.img`; the kernel writes back any remaining dirty cache
and shares the extents. (5) resume, thaw. (6) background: `Base` (reflink the
committed top-layer file into `bases/<digest>.img` if not cached, hash it),
`Delta` (FIEMAP both, emit ranges whose physical address differs, plus holes),
zstd, package, push. (7) release the capture.

**Snapshot, stop.** Same to step 4; the VM stays paused while steps 6 runs
into the local package store; stop the VMM once the package is verified; push
afterwards.

**Clone restore, base not cached.** (1) resolve and pin the package digest,
fetch the manifest and config, check compatibility, policy, signature. (2) pull
the base artifact by its qualified reference, verify, store as
`bases/<digest>.img` under lease `flintlock/base/<digest>`. (3) per volume:
reflink base into `vm/<clone>/volumes/<id>` (or `View`-like read-only link),
`pwrite` delta ranges, punch holes for zero ranges, fsync. (4) network, then
VMM load with `cmd.Dir` set (Firecracker) or rewritten `config.json` (CH).
(5) resume, thaw if quiesced.

**Clone restore, base cached.** Skip step 2.

**Delete source VM.** Volume release step removes `volumes/` links; lease
release lets containerd remove the snapshot file (rename to `rm-<id>`, unlink).
Captures are already released; a capture still referenced by an in-progress
snapshot blocks deletion until the snapshot finishes or fails.

**Delete clone.** Same, plus the clone's volume files (not containerd's) are
unlinked by the release step; the base stays under its own lease.

**Delete snapshot record.** Remove `packages/<snap>` content not referenced by
a clone lease (memory file in lazy mode); remove leftover captures.

**GC.** Periodic sweep: `bases/*.img` with no snapshot record or clone
referencing the digest and older than a retention window are unlinked;
`captures/` entries without a record are unlinked; containerd `rm-*` files
older than an hour are reported (containerd does not sweep them).

**Daemon restart.** Snapshot in `capturing` or earlier: resume the VM, delete
captures. In `packaging`: if the local package is complete and verified, stop
the VM if requested and continue to push; otherwise fail, resume, delete
captures. Restore in progress: remove the partial clone directory and files.

## 7. Failure modes

| Step | Failure | State left | Handling |
| ---- | ------- | ---------- | -------- |
| Prepare (reflink) | filesystem without reflink support: falls back to a full copy, O(size) | slow start, no corruption | provisioning validates the filesystem; warn if a `Prepare` exceeds a threshold |
| FICLONE | `EXDEV` (capture dir on another filesystem), `EOPNOTSUPP`, `ENOSPC` | no capture | fail the snapshot, resume VM; configuration error surfaced |
| FICLONE under dirty cache | long writeback inside the pause (131 ms with ~200 MiB dirty in experiment B/C-1) | longer pause | fsync before pause where the VMM does not; account under SNAP-VOL-005 |
| Delta computation | FIEMAP returns unwritten or shared-but-moved extents | over-inclusive delta | correctness preserved (extra ranges only); size grows |
| Base export | top-layer file changed since the VM was created (should never happen: committed snapshots are immutable) | digest mismatch | abort; record which base digest the VM was created from at `Create` time in the ledger |
| Restore pwrite | `ENOSPC` mid-way | partial clone file | remove clone dir, fail restore (SNAP-RST-013) |
| VMM load | path mismatch | VMM exits | remove partial state, fail |
| containerd GC removes the top-layer file while the base export reads it | cannot happen: the image's snapshots are kept alive by the source VM's lease; the base cache takes its own lease before the VM is deleted | | |
| Crash during containerd `Remove` | `rm-<id>` file left | disk use | GC report |

## 8. Base block image

* **What it is.** The committed top-layer file of the image as unpacked by
  this host's containerd: `<root>/snapshots/<committed id>`. It is a full ext4
  image (every layer file holds the cumulative contents,
  [`blockfile.md`](https://github.com/containerd/containerd/blob/main/docs/snapshotters/blockfile.md)).
  Because containerd builds it per host, two hosts produce different bytes for
  the same OCI image; the base digest is therefore the sha256 of the file
  bytes, not the OCI image digest.
* **Export.** `Base` reflinks the file into `bases/<digest>.img` (O(extents)),
  hashes it (O(size), once per host and image), and pushes it as an OCI
  artifact (`application/vnd.flintlock.base-block-image.v1`) with a single
  zstd layer written sparse (SEEK_DATA/SEEK_HOLE ranges). The snapshot package
  records `repository@digest` (SNAP-PKG-006) and links it as a referrer
  (SNAP-PKG-007).
* **Cache and verification on the target.** Pull once into
  `bases/<digest>.img`, verify sha256, hold lease `flintlock/base/<digest>`
  with a refcount of clones in the ledger. Reads of the base by clones are
  reflinks, so the cache costs one copy of the image per host.
* **GC.** Unreferenced bases are removed after a retention window; the
  artifact remains in the registry.
* **Count.** One base per (host, image) on the source side. A fleet with N
  hosts creating VMs from the same image produces up to N bases; clones of a
  given snapshot always need the one base their source used. Option C's
  deterministic base removes this multiplicity; Option B accepts it.

## 9. Delta format

Block delta v1: a zstd-compressed stream of

```text
header: magic "FLDELTA1", volume size, block size (4096), base digest or zero
record: offset (8 bytes), length (8 bytes), kind (data | zero), data bytes
```

Ranges are 4 KiB aligned and coalesced. Producer: FIEMAP (with
`FIEMAP_FLAG_SYNC`) on the capture and on the base clone; a 4 KiB block is
unchanged only if both files map it to the same physical address; a hole or
`FIEMAP_EXTENT_UNWRITTEN` extent in the capture where the base has data becomes
a `zero` record
([fiemap.rst](https://docs.kernel.org/filesystems/fiemap.html)). Experiment B-3
recovered the exact changed set with three over-included blocks and none
missed. The same stream with a zero base digest carries a full sparse image.
Consumer: `pwrite` data records; `fallocate(PUNCH_HOLE)` zero records.

Size: O(changed blocks) plus compression. `FIEMAP_EXTENT_SHARED` is not used:
it means "refcount > 1 with any file", and a fresh capture shares every extent
with the live volume (66% still shared after 300 ms of writes in B-3).

## 10. Experiments

Host: Linux 7.2.5, `/home` Btrfs (`compress=zstd:3`), Go program
`~/.local/share/flintlock-experiments/reflink/main.go` (stdlib only: FICLONE,
FIEMAP, SEEK_DATA/SEEK_HOLE via raw ioctls). 256 MiB files, 4 KiB blocks each
carrying its block number and a sequence number.

```text
== filesystem magic 0x9123683e (btrfs)
== B/C-2 io.Copy (copy_file_range): 268435456 bytes in 994.844µs; blocks with identical physical address: 65536/65536
== FICLONE base->live took 65.242µs
== B/C-1 FICLONE of a file under concurrent writes took 131.187857ms (writer had touched 53180 distinct blocks at clone time, 59420 at end)
== B/C-1 capture check: 53180 blocks differ from base, 0 torn/inconsistent blocks, 32737 blocks where live diverged after the clone (independence)
== B-3 capture FIEMAP: 55388 extents, SHARED covers 178343936/268435456 bytes (66.4%)
== B-3 physical-address delta vs base: 53183 blocks differ; actual content changes: 53180; missed=0 extra=3
== B/C-4 SEEK_DATA/SEEK_HOLE ranges: [1048576,1056768) [10485760,10493952) [41943040,41951232)
== sparse.img apparent 64 MiB, allocated 24 KiB
```

What this confirms:

* Go's `io.Copy` reflinks on Btrfs: 256 MiB in 1 ms with all 65536 blocks at
  the same physical address as the source. `Prepare` is a metadata operation.
* FICLONE of an idle file is 65 µs. Under a writer that had dirtied about
  200 MiB, it took 131 ms: the kernel's writeback before remapping
  ([`remap_range.c`](https://github.com/torvalds/linux/blob/master/fs/remap_range.c)).
  The pause cost is the dirty page cache, not the file size, so fsync before
  the pause keeps it near the idle figure.
* The capture is consistent (no torn blocks) and independent of later writes.
* The physical-address comparison yields the delta exactly; the shared flag
  does not.
* `st_blocks` reports the full size for every reflinked file, so disk usage
  must be measured at the filesystem level, not per file.

XFS repeat (B-5): the same program on a 4 GiB loop-backed XFS made with
`mkfs.xfs -m reflink=1`, no compression.

```text
reflink=1
== filesystem magic 0x58465342 (xfs)
== B/C-2 io.Copy (copy_file_range): 268435456 bytes in 1.010995ms; blocks with identical physical address: 65536/65536
== FICLONE base->live took 47.189µs
== B/C-1 FICLONE of a file under concurrent writes took 485.817217ms (writer had touched 25718 distinct blocks at clone time, 29665 at end)
== B/C-1 capture check: 25718 blocks differ from base, 0 torn/inconsistent blocks, 6422 blocks where live diverged after the clone (independence)
== B-3 capture FIEMAP: 32996 extents, SHARED covers 258297856/268435456 bytes (96.2%)
== B-3 physical-address delta vs base: 25718 blocks differ; actual content changes: 25718; missed=0 extra=0
== B/C-4 SEEK_DATA/SEEK_HOLE ranges: [1048576,1056768) [10485760,10493952) [41943040,41951232)
== allocated bytes: base=268435456 live=428556288 capture=268951552 copyfilerange=268435456
```

On XFS the physical-address delta was exact (no extras), and `st_blocks` does
account for copy-on-write: the live file grew to 409 MiB after the writer
rewrote 29665 blocks while the capture stayed at 256 MiB plus metadata. The
FICLONE under writes took 486 ms with about 100 MiB dirty, slower than on
Btrfs for a similar amount of dirty data because the loop-backed XFS wrote
through a file on Btrfs; the shape of the cost (writeback, not size) is the
same. Cloud Hypervisor fsync behaviour on `vm.snapshot` remains unverified
because Cloud Hypervisor is not installed on this host.

## 11. Effort and migration

* Code: new port and `infrastructure/volume/blockfile` (FICLONE, FIEMAP, range
  stream, ledger), `convert.go` case, `View` for read-only volumes, flag and
  wiring, volume release step, Firecracker `cmd.Dir` and relative drive paths,
  CH `readonly`, provisioning mode, e2e runner switch. No new Go module
  dependencies beyond `golang.org/x/sys` (already present) and bolt (already
  present via containerd).
* Host migration: hosts move from a thin pool to a reflink directory. Existing
  VMs on devmapper keep running under the devmapper snapshotter until
  recreated; both snapshotters can be configured at once and the flag selects
  the one for new VMs. Snapshots are only offered for blockfile volumes.
* Firecracker VMs created before the change have absolute drive paths;
  restoring their snapshots as several clones on one host is not possible.
  The requirements say every restore creates new paths, which holds only for
  VMs created with relative paths, so such snapshots are rejected at creation
  (SNAP-CRT-004 style check on the drive path form).
* Upstream: none required. Nice to have: containerd sparse-copy PR #12956 and
  a snapshotter capture operation (PR #13111 precedent) so captures become
  containerd snapshots.

## 12. Risks and open questions

* The blockfile snapshotter is young and thin: no crash sweep of `rm-*`
  files, apparent-size `Usage`, non-sparse copies on non-reflink hosts.
* Fixed volume size equal to the scratch file; `Volume.Size` (unused today)
  could select among several scratch files or trigger a `resize2fs` at first
  boot.
* Btrfs `compress=` changes FIEMAP physical semantics (extents are compressed
  units); the comparison still worked in B-3 but should be re-verified on an
  uncompressed Btrfs and on XFS (B-5).
* Cloud Hypervisor does not document an fsync on `vm.snapshot`; flintlock
  fsyncs each volume file before the FICLONE. Verify with a running CH.
* A capture is a full-size file sharing extents; on a filesystem near full,
  CoW on the live volume after capture needs free space.

## 13. References

* containerd blockfile: <https://github.com/containerd/containerd/blob/main/plugins/snapshots/blockfile/blockfile.go>,
  <https://github.com/containerd/containerd/blob/main/docs/snapshotters/blockfile.md>,
  <https://github.com/containerd/containerd/blob/release/1.7/snapshots/blockfile/plugin/plugin.go>,
  <https://github.com/containerd/containerd/pull/12956>, <https://github.com/containerd/containerd/pull/13111>
* containerd apply/mount: <https://github.com/containerd/containerd/blob/main/core/diff/apply/apply_linux.go>,
  <https://github.com/containerd/containerd/blob/main/core/mount/mount_linux.go>
* Kernel: <https://man7.org/linux/man-pages/man2/ioctl_ficlone.2.html>,
  <https://man7.org/linux/man-pages/man2/copy_file_range.2.html>,
  <https://man7.org/linux/man-pages/man2/lseek.2.html>,
  <https://docs.kernel.org/filesystems/fiemap.html>,
  <https://github.com/torvalds/linux/blob/master/fs/read_write.c>,
  <https://github.com/torvalds/linux/blob/master/fs/remap_range.c>,
  <https://github.com/torvalds/linux/blob/master/fs/iomap/fiemap.c>,
  <https://github.com/torvalds/linux/blob/master/fs/xfs/xfs_iomap.c>,
  <https://github.com/torvalds/linux/blob/master/fs/btrfs/fiemap.c>,
  <https://github.com/torvalds/linux/blob/master/fs/btrfs/file.c>
* Go: <https://github.com/golang/go/blob/master/src/internal/poll/copy_file_range_linux.go>
* XFS: <https://man7.org/linux/man-pages/man8/mkfs.xfs.8.html>,
  <https://git.kernel.org/pub/scm/fs/xfs/xfsprogs-dev.git/tree/doc/CHANGES>,
  <https://man7.org/linux/man-pages/man8/xfs_io.8.html>
* Firecracker: <https://github.com/firecracker-microvm/firecracker/blob/main/src/vmm/src/devices/virtio/block/virtio/device.rs>,
  <https://github.com/firecracker-microvm/firecracker/blob/main/docs/api_requests/block-io-engine.md>,
  <https://github.com/firecracker-microvm/firecracker/blob/main/docs/api_requests/block-caching.md>,
  <https://github.com/firecracker-microvm/firecracker/blob/main/docs/api_requests/block-discard.md>,
  <https://github.com/firecracker-microvm/firecracker/blob/main/docs/api_requests/patch-block.md>,
  <https://github.com/firecracker-microvm/firecracker/blob/main/docs/snapshotting/snapshot-support.md>
* Cloud Hypervisor: <https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/vmm/src/config.rs>,
  <https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/vmm/src/vm_config.rs>,
  <https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/block/src/factory.rs>,
  <https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/docs/api.md>,
  <https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/release-notes.md>
* Flintlock: `core/ports/services.go`, `core/steps/runtime/volume_mount.go`,
  `core/plans/{microvm_create_update,microvm_delete}.go`,
  `infrastructure/containerd/{image_service,convert,config}.go`,
  `infrastructure/microvm/firecracker/create.go`,
  `infrastructure/microvm/cloudhypervisor/create.go`,
  `internal/inject/wire.go`, `internal/command/flags/flags.go`,
  `internal/provision/containerd.go`, `test/e2e/utils/runner.go`
