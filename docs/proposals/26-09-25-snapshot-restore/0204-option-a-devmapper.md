# 0204. Option A: stay on devmapper (dm-thin)

* Status: proposed design candidate
* Date: 2026-09-25
* Authors: @richardcase
* Issue: [#204](https://github.com/liquidmetal-dev/flintlock/issues/204)
* Companions: [requirements](0204-snapshot-restore-requirements.md),
  [options survey](0204-root-volume-options.md),
  [Option B](0204-option-b-blockfile.md), [Option C](0204-option-c-ro-base-rw-disk.md)

## 1. Summary

Volumes stay as today: containerd's devmapper snapshotter `Prepare`s a thin
device from the committed image snapshot and the VMM opens
`/dev/mapper/<pool>-snap-N`. A snapshot captures the volume by suspending the
VM's thin device, sending `create_snap` to the pool, and resuming, all inside
the VMM pause. The capture is a new thin device that flintlock owns. The delta
is computed with `thin_delta` against the image's committed device under a
metadata snapshot, read from the capture device in whole pool blocks, and
shipped as block ranges. The base is the committed image device, exported
once per host as a sparse raw artifact. On the target, the base is attached as
a read-only external origin (a loop device over the base file), and each clone
is a thin device on that origin with the delta written in.

The design's hard parts are all about containerd: it allocates thin device IDs
from its own database with no reservation mechanism, it never learns about
devices flintlock creates, and its pools run with block zeroing disabled.

## 2. Requirements mapping

| Requirement | How this design meets it |
| ----------- | ------------------------ |
| SNAP-VOL-001 | `create_snap` shares data blocks byte for byte; ranges are transferred in whole `data_block_size` blocks so the target device is byte-identical, including any stale bytes a partial write left in a block (section 9). |
| SNAP-VOL-003 | suspend, `create_snap`, resume run inside the pause; suspend flushes the device page cache (`sync_blockdev` via lockfs). |
| SNAP-VOL-004/005 | suspend cost is in-flight I/O, not size; `create_snap` is a metadata operation. |
| SNAP-VOL-006 | attach base as external origin, `create_thin`, `pwrite` whole blocks, activate. |
| SNAP-VOL-007 | activated clone devices get a per-VM name and are linked at `vm/<clone>/volumes/<id>`; Firecracker opens the relative link from its working directory; CH gets the absolute path in `config.json`. |
| SNAP-VOL-008 | the source's containerd snapshot is untouched; the capture is a sibling thin device with a flintlock-owned ID. |
| SNAP-VOL-009 | block delta v1 in volume offsets with block size recorded; conversion between pools with different `data_block_size` is exact because ranges are in bytes. |
| SNAP-VOL-010 | read-only volumes become external-origin thins on the target with no delta (or `View`-equivalent). |
| SNAP-VOL-011 | base artifact digest = sha256 of the exported sparse image; verified on pull. |
| SNAP-VOL-012 | flintlock ledger of device IDs, captures, bases, clones; GC deletes thin devices with `delete <id>`. |
| SNAP-CRT-005/007 | capture inside pause; delta and packaging afterwards or during the held pause for stop. |
| SNAP-CLN-001/002 | one thin device per clone on the shared origin. |
| SNAP-API-005/006, SNAP-OPS-004 | as in Option B, with `dmsetup remove` plus pool `delete` for cleanup. |

## 3. Host and image contract

* dm-thin pool as today (`hack/scripts/devpool.sh`, `direct_lvm.sh`,
  `internal/provision/{devpool,lvm}.go`), plus thin-provisioning-tools on the
  host (`thin_delta`, `thin_dump`, `thin_ls`; already in `AptPackages`).
* `thin_delta` and `thin_dump` operate on the pool metadata device and need a
  reserved metadata snapshot on a live pool
  ([`thin_delta(8)`](https://github.com/jthornber/thin-provisioning-tools/blob/main/man8/thin_delta.txt)).
* **Pool zeroing.** Both flintlock pool recipes disable zeroing:
  `skip_block_zeroing` (`devpool.sh:67`, `internal/provision/devpool.go:92`)
  and `lvconvert --zero n` (`direct_lvm.sh`). With zeroing off, a first write
  that covers part of a block leaves the rest of the block holding whatever
  the pool's data device held there
  ([`dm-thin.c`](https://github.com/torvalds/linux/blob/master/drivers/md/dm-thin.c)
  `schedule_zero`). The design keeps byte identity by moving whole blocks, but
  the stale bytes are also an information leak between VMs that share a pool;
  the design recommends enabling zeroing (see section 12).
* Loop device support for external origins (`losetup -r`).
* Guest images unchanged. Kernel and initrd stay on `native`.

## 4. On-host layout

```text
<StateRootDir>/
├── vm/<ns>/<name>/<uid>/volumes/<id>   -> /dev/mapper/flintlock-<uid>-<id>  (clones) or
│                                       -> /dev/mapper/<pool>-snap-N          (containerd-created)
├── bases/<digest>.img                  # sparse raw base images (external origins), read-only
├── captures/<snapshot-id>/             # ledger entries only; devices live in the pool
├── packages/<snapshot-id>/
└── volumes.db                          # ledger: device ids, captures, bases, clones
/dev/mapper/<pool>                      # shared with containerd
/dev/mapper/<pool>-snap-N               # containerd devices
/dev/mapper/flintlock-cap-<snap>-<id>   # captures (activated only while exporting)
/dev/mapper/flintlock-<uid>-<id>        # clone volumes
/dev/loopN                              # read-only base, one per cached base in use
```

## 5. Flintlock design

The `VolumeService` port is the one defined in
[Option B, section 5.1](0204-option-b-blockfile.md#51-new-port-volumeservice);
this option implements it in `infrastructure/volume/devmapper`.

### 5.1 Device ID management

containerd allocates 24-bit IDs from a bolt bucket, reusing the lowest free ID
and otherwise taking the next sequence value; there is no reservation API, no
range setting, and a message failure caused by a duplicate ID marks that ID
faulty permanently
([`metadata.go`](https://github.com/containerd/containerd/blob/v1.7.35/snapshots/devmapper/metadata.go),
[`pool_device.go`](https://github.com/containerd/containerd/blob/v1.7.35/snapshots/devmapper/pool_device.go)).
Flintlock therefore allocates from the top of the 24-bit space downwards
(`0xFFFFFF`, `0xFFFFFE`, ...), recorded in its ledger, and refuses to create a
device if containerd's sequence has ever approached the flintlock range (read
from `dmsetup ls` device tables at startup). containerd's sequence only grows
with the number of snapshots ever created; a host would need about 16 million
snapshots to collide. This is a convention, not a guarantee, and is the main
reason to prefer an upstream containerd operation (section 12).

### 5.2 containerd adapter

* `Create`: unchanged, `ImageService.PullAndMount`; the returned
  `/dev/mapper/<pool>-snap-N` is linked at `volumes/<id>`. Read-only volumes
  switch to `View`.
* The adapter records, at `Create`, the parent (committed image device) ID
  and the pool's `data_block_size` in the ledger, because the base must be
  exported from exactly that device.
* `Capture`: `dmsetup suspend <vm dev>`; `dmsetup message <pool> 0
  "create_snap <cap-id> <vm-id>"`; `dmsetup resume <vm dev>`. Then
  `dmsetup create flintlock-cap-<snap>-<id> --table "0 <sectors> thin <pool>
  <cap-id>"`. containerd suspends the same way when it prepares children
  ([`CreateSnapshotDevice`](https://github.com/containerd/containerd/blob/v1.7.35/snapshots/devmapper/pool_device.go)).
* `Base`: the committed image device is inactive after Commit (containerd
  deactivates it). Activate it under a flintlock name, `reserve_metadata_snap`,
  `thin_dump -m --dev-id <base-id>` for its mapped ranges, `pread` those ranges
  into `bases/<digest>.img` (sparse file), `release_metadata_snap`, deactivate,
  hash. Do not leave the base active: containerd would suspend and resume it
  on every child `Prepare`, and a forced `RemoveDevice` on GC would fail
  flintlock's reads with EIO.
* `Delta`: `reserve_metadata_snap`; `thin_delta -m --thin1 <base-id> --thin2
  <cap-id> <tmeta>`; for `different` and `right_only` ranges read the bytes
  from the capture device; `left_only` ranges become `zero` records (the
  capture has the block unmapped, which reads as zero, `provision_block` in
  `dm-thin.c`); `release_metadata_snap`. Units are thin blocks
  (`data_block_size` sectors); the stream records byte offsets.
* `Restore`: ensure `bases/<digest>.img` is present; `losetup -r` it (one loop
  per base per host, refcounted in the ledger); `create_thin <clone-id>`;
  `dmsetup create flintlock-<uid>-<id> --table "0 <sectors> thin <pool>
  <clone-id> /dev/loopN"`; `pwrite` data records in whole target-pool blocks
  (pad a record to block boundaries with bytes from the base if the source
  pool's block size was smaller); zero records: `blkdiscard` whole blocks or
  write zeros. Every later `create_snap` descendant must carry the same
  origin parameter (kernel doc).
* `Release`: `dmsetup remove`, pool `delete <id>`, ledger update; drop the
  loop device when the last user of a base goes.

### 5.3 Steps, plans, providers, provisioning

As Option B sections 5.3 to 5.6, with these differences: the provider links
point at device nodes; provisioning adds nothing new for storage but the
containerd config emitted by `BuildContainerdConfig` should stop passing
`skip_block_zeroing` when the design is adopted, and `thin-provisioning-tools`
becomes a runtime dependency of flintlockd, not just of provisioning.

## 6. Lifecycle flows

**First VM start, image not on host.** (1) pull. (2) unpack: `create_thin`,
mkfs (eager inode tables), untar, Commit per layer (O(image)). (3) `Prepare`:
suspend parent if active, `create_snap`, resume, activate (metadata). (4) link,
boot. First writes pay a 64 KiB copy-on-write per shared block.

**Snapshot, resume.** (1) quiesce optional. (2) pause. (3) VMM snapshot
(Firecracker fsyncs the block device: `blkdev_fsync` writes back and issues a
flush). (4) `Capture` per volume: suspend (flush + drain), `create_snap`,
resume. (5) resume VMM. (6) background: `Base` if not cached (O(base) read,
once per host and image), `Delta` (metadata walk + O(changed) read), package,
push, release the capture device.

**Snapshot, stop.** Same to (4); hold the pause; run (6) into the local store;
stop the VMM; push.

**Clone restore, base not cached.** (1) pin digest, checks. (2) pull base to
`bases/<digest>.img`, verify. (3) `losetup -r`. (4) per volume `create_thin`,
activate with origin, write delta. (5) network, VMM load, resume.

**Clone restore, base cached.** Skip (2); (3) only if no loop exists.

**Delete source VM.** Release links; lease release lets containerd remove its
snapshot. Any capture still active blocks deletion until the snapshot record
finishes.

**Delete clone.** `dmsetup remove`, pool `delete`, ledger; base loop released
when unreferenced.

**Delete snapshot record.** Package content per SNAP-API-005; capture device
removed if still present.

**GC.** Ledger sweep of captures and clones with no owner; `dmsetup ls` scan
for `flintlock-*` devices absent from the ledger (crash leftovers) which are
removed; bases past retention.

**Daemon restart.** As Option B; in addition, flintlock re-reads `dmsetup ls`
to rebuild the active-device view and releases metadata snapshots left held
(`dmsetup status <pool>` shows a held root).

## 7. Failure modes

| Step | Failure | State left | Handling |
| ---- | ------- | ---------- | -------- |
| `create_snap` | duplicate ID (collision with containerd) | message fails; if containerd sent it, that ID is faulty for good | flintlock allocates from the top; on failure pick the next ID down and log |
| suspend | VMM I/O stalls for the duration; never EIO | inside the pause anyway | bounded by in-flight I/O; measure (A-1b) |
| `reserve_metadata_snap` | a snapshot is already held (crash) | tools fail | release at startup; serialise metadata snaps with a host-wide lock |
| `thin_delta` | pool metadata large: walk cost | slow packaging | off the critical path |
| base export | committed device active in containerd's view | containerd suspends it on child Prepare | flintlock activates under its own name only for the export and deactivates after |
| containerd GC removes an image snapshot flintlock has open | forced deactivate: EIO | lease of the source VM prevents it while the VM exists; base export completes before VM deletion or fails | ledger records base digest at `Create` |
| target pool with different `data_block_size` | records not block aligned | partial first writes leave stale bytes with zeroing off | pad from the base; require zeroing on target pools |
| external origin file changes | corrupt clones | must never happen | file is 0400, digest verified, never opened for write |
| loop device exhaustion | `losetup` fails | no clone | one loop per base, not per clone; `max_loop` sizing in docs |
| daemon crash between `create_snap` and ledger write | orphan device | pool space | startup scan of `flintlock-*` names |

## 8. Base block image

* **What.** The bytes of the committed image device as built by this host's
  containerd (`mkfs` + untar per layer), read as mapped ranges from
  `thin_dump`; unmapped ranges are holes and read as zero on both sides.
  Digest = sha256 of the full sparse image. Different hosts produce different
  bases for the same OCI image, exactly as in Option B.
* **Export.** Once per (host, image), O(base size) read, from an inactive
  committed device under a temporary activation. Pushed as a
  `application/vnd.flintlock.base-block-image.v1` artifact, zstd, sparse.
* **Target.** Pulled to `bases/<digest>.img`, verified, attached read-only as
  a loop device, used as external origin: no import into the pool, no copy.
  Clones' unprovisioned blocks read through to the origin; reads past the
  origin's end return zero (`thin_preresume`, `process_cell`).
* **GC.** Ledger refcount of clones per base; remove the file and loop when
  zero and past retention.

## 9. Delta format

Block delta v1 as in Option B section 9, with block size = source pool
`data_block_size` in bytes (64 KiB for flintlock's pools) and every record
aligned to it. Producer: `thin_delta` ranges (`different`, `right_only` → data
read from the capture device; `left_only` → zero). `thin_delta`'s `different`
means "mapped to different data blocks" and may include blocks whose bytes are
equal; that only inflates the stream.

Whole-block transfer is what makes the target byte-identical when zeroing is
off: a partial write on the target would otherwise leave stale target-pool
bytes where the source had stale source-pool bytes.

## 10. Experiments

Scripts: `~/.local/share/flintlock-experiments/scripts/exp-a-dmthin.sh`
(A-1 stale data with and without zeroing, suspend/`create_snap` timing under a
live writer, A-2 `thin_delta`/`thin_dump`/`thin_ls` under a metadata snap,
SEEK_HOLE/punch-hole/`blkdiscard` on a thin device, A-3 external origin
pass-through, external copy of a partial block, snapshot of an external-origin
thin) and `exp-a4-containerd.sh` (A-4 containerd devmapper snapshotter sharing
a pool with out-of-band devices, including a deliberate ID collision).

Host: Linux 7.2.5, dmsetup library 1.02.216, thin-provisioning-tools 1.3.3,
containerd 2.3.5 (for A-4). Loop-backed pool, 64 KiB blocks
(`data_block_size` 128), `skip_block_zeroing`, data device pre-filled with
0xAB so stale bytes are recognisable.

A-1b, suspend and `create_snap` while a writer issues random 64 KiB
direct-I/O writes to the device:

```text
suspend=80ms create_snap=3ms resume=9ms
writer still running after suspend/resume (stalled, no error)
0 1048576 thin 728320 1025279     # dmsetup status of the VM device after resume
0 1048576 thin 588928 1025279     # the capture: mapped sectors frozen at snapshot time
```

The writer stalled for the suspend and continued; the capture's mapped-sector
count stayed at the value at `create_snap` while the origin kept growing.

A-2, tools under a reserved metadata snapshot (pool status shows the held root
`1024`):

```text
0 2097152 thin-pool 0 1698/16384 7257/16384 95 rw discard_passdown queue_if_no_space - 1024
<superblock uuid="" time="1" transaction="0" data_block_size="128" nr_data_blocks="0">
  <diff left="1" right="2">
    <same begin="0" length="1"/>
    <left_only begin="2" length="8"/>
    <different begin="10" length="8"/>
    ...
<device dev_id="2" mapped_blocks="4601" transaction="0" creation_time="1" snap_time="1">
    <single_mapping origin_block="0" data_block="0" time="0"/>
    <range_mapping origin_begin="10" data_begin="3638" length="9" time="0"/>
    ...
DEV MAPPED CREATE_TIME SNAP_TIME
  1 356MiB           0         1
  2 288MiB           1         1
```

`thin_delta` reports ranges in thin blocks (`begin`, `length` in units of
`data_block_size`); `left_only` here means the origin has since written blocks
the capture does not map. `thin_dump --dev-id 2` lists the capture's mapped
ranges by virtual block (`origin_begin`), which is what the base export reads.

A-2b, hole handling on the thin device:

```text
os.lseek(fd, 0, SEEK_HOLE) -> OSError: [Errno 22] Invalid argument
fallocate --punch-hole: fallocate failed: keep size mode is unsupported
blkdiscard --offset 0 --length 65536: OK
0 1048576 thin 588800 1025279     # mapped sectors dropped by 128 (one 64 KiB block)
```

So `SEEK_HOLE` is rejected outright on a block device on this kernel (not
"never finds a hole"), punch-hole is unsupported, and `blkdiscard` of a whole
block unmaps it.

A-3, external origin (`losetup -r` over a 256 MiB random file):

```text
first 8 MiB identical to origin (pass-through reads)
rest of the 64 KiB block was copied from origin (schedule_external_copy)
snapshot of external-origin thin is identical
0 524288 thin 128 2175            # only the one written block is mapped
```

A 4 KiB write at 1 MiB provisioned one block whose other 60 KiB were copied
from the origin, and a `create_snap` descendant activated with the same origin
parameter read identically.

A-4, containerd 2.3.5 devmapper snapshotter sharing the pool: with an
out-of-band thin device at ID 16000000 already present, containerd started,
allocated ID 1 for its first snapshot and worked normally; `ctr snapshots ls`
showed only its own snapshot while `dmsetup ls` showed both:

```text
exp-ctrpool-oob: 0 262144 thin 253:1 16000000
exp-ctrpool-snap-1: 0 2097152 thin 253:1 1
```

The image pull failed in this environment (`ctr` on containerd 2.3.5:
"unable to initialize unpacker: no unpack platforms defined" after "Unpack
configuration not supported, skipping" for devmapper), so the snapshot was a
parentless one. After `commit`, containerd deactivated the device: `dmsetup
ls` no longer listed it, as the source predicts. The collision test then sent
`create_thin 1` out of band while containerd's committed device already held
ID 1:

```text
device-mapper: message ioctl on exp-ctrpool  failed: File exists
Command failed.
KEY   PARENT KIND
base1        Committed
vm2   base1  Active
vm3   base1  Active
exp-ctrpool-oob: 0 262144 thin 253:1 16000000
exp-ctrpool-snap-2: 0 2097152 thin 253:1 2
exp-ctrpool-snap-3: 0 2097152 thin 253:1 3
```

The kernel rejects a duplicate ID with `EEXIST`, so an out-of-band creator
gets a clear error and can retry with another ID, and containerd continued
with IDs 2 and 3. The reverse case (containerd colliding with a flintlock ID
and marking it faulty) was not exercised; it rests on the source reading of
`MarkFaulty` in `pool_device.go`.

A-1a and A-1c, a 4 KiB first write at offset 0 on a fresh thin device, data
device pre-filled with 0xAB:

```text
### skip_block_zeroing: table: 0 524288 thin-pool 7:2 7:1 128 32768 1 skip_block_zeroing
### skip_block_zeroing: unprovisioned read at offset 1 MiB:      00 00 00 00 ...
    bytes 0..    (written):                          43 f0 56 be 89 63 68 95 ...
    bytes 4096.. (rest of the same 64 KiB block):    ab ab ab ab ab ab ab ab ...
    bytes 65536.. (next block, unprovisioned):        00 00 00 00 ...
### zeroing_on: table: 0 524288 thin-pool 7:2 7:1 128 32768 0
    bytes 4096.. (rest of the same 64 KiB block):    00 00 00 00 ...
```

With flintlock's pool settings the guest reads the pool's previous contents
in the rest of a partially written block; with zeroing on it reads zeros.
Unprovisioned blocks read as zero in both cases. This confirms both the
whole-block transfer rule in section 9 and the information-leak concern in
section 12.

## 11. Effort and migration

* Code: `infrastructure/volume/devmapper` (dmsetup and thin tools wrappers,
  ledger, range stream), the shared `VolumeService` port and steps, provider
  path changes as Option B. Runtime dependency on `dmsetup` and
  thin-provisioning-tools binaries.
* Hosts: no storage migration; existing pools work. Recommended change:
  enable block zeroing on new pools (provisioning scripts and Go provisioner).
* Existing VMs: snapshottable as-is (their devices are containerd-created and
  the base can be exported from their parent). Same-host fan-out needs the
  relative-path change for Firecracker, as in Option B.
* Upstream: a containerd snapshotter operation to snapshot an active snapshot
  would remove the ID convention and the ledger for captures
  (PR #13111 precedent).

## 12. Risks and open questions

* **Out-of-band devices in containerd's pool.** ID collisions are avoided by
  convention only; a collision permanently marks an ID faulty on containerd's
  side. A separate pool for flintlock devices would need a second
  snapshotter instance or an unrelated pool, doubling storage management.
* **Zeroing disabled** leaks stale pool data into guests and complicates
  partial-block application. Enabling it costs a zero write per newly
  provisioned block on first partial write.
* **Metadata snapshot is pool-global.** Only one can be held; concurrent
  snapshots on a host serialise on it.
* **Forced removal on GC** can EIO a device flintlock has open if the ledger
  and containerd's lease disagree. The base export must complete under the
  source VM's lease.
* **LVM-managed pools** cannot share the pool with containerd's ID allocator
  (`lvmthin(7)`, transaction-id check), so this design drives dmsetup
  directly even on LVM-provisioned pools.
* Open: whether to run a second containerd snapshotter instance on a
  flintlock-owned pool purely for captures.

## 13. References

* containerd v1.7.35 devmapper: <https://github.com/containerd/containerd/tree/v1.7.35/snapshots/devmapper>
  (`metadata.go`, `pool_device.go`, `snapshotter.go`, `device_info.go`,
  `config.go`),
  <https://github.com/containerd/containerd/blob/main/docs/snapshotters/devmapper.md>,
  <https://github.com/containerd/containerd/pull/13111>
* Kernel: <https://github.com/torvalds/linux/blob/master/Documentation/admin-guide/device-mapper/thin-provisioning.rst>,
  <https://github.com/torvalds/linux/blob/master/drivers/md/dm-thin.c>,
  <https://github.com/torvalds/linux/blob/master/drivers/md/dm.c>,
  <https://github.com/torvalds/linux/blob/master/block/bdev.c>,
  <https://github.com/torvalds/linux/blob/master/block/fops.c>,
  <https://github.com/torvalds/linux/blob/master/block/blk-lib.c>
* dmsetup(8): <https://man7.org/linux/man-pages/man8/dmsetup.8.html>;
  lvmthin(7): <https://man7.org/linux/man-pages/man7/lvmthin.7.html>;
  lseek(2): <https://man7.org/linux/man-pages/man2/lseek.2.html>
* thin-provisioning-tools: <https://github.com/jthornber/thin-provisioning-tools>
  (`man8/thin_delta.txt`, `man8/thin_dump.txt`, `man8/thin_ls.txt`,
  `man8/thin_migrate.txt`, `src/thin/delta_visitor.rs`, `src/thin/xml.rs`)
* lvm2: <https://github.com/lvmteam/lvm2> (`libdm/libdm-deptree.c`,
  `lib/metadata/thin_manip.c`)
* Firecracker: <https://github.com/firecracker-microvm/firecracker/blob/main/docs/snapshotting/snapshot-support.md>,
  <https://github.com/firecracker-microvm/firecracker/blob/main/docs/api_requests/block-caching.md>,
  <https://github.com/firecracker-microvm/firecracker/tree/main/src/vmm/src/devices/virtio/block/virtio>
* Flintlock: `hack/scripts/devpool.sh`, `hack/scripts/direct_lvm.sh`,
  `internal/provision/{devpool,lvm,defaults,containerd}.go`,
  `infrastructure/containerd/{image_service,convert}.go`
