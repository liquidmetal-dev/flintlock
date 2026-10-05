# 0204. Root Volume Decision: implement Option C directly

* Status: proposed
* Date: 2026-09-26
* Authors: @richardcase
* Issue: [#204](https://github.com/liquidmetal-dev/flintlock/issues/204)
* Companions: [requirements](0204-snapshot-restore-requirements.md),
  [options survey](0204-root-volume-options.md),
  [Option A](0204-option-a-devmapper.md), [Option B](0204-option-b-blockfile.md),
  [Option C](0204-option-c-ro-base-rw-disk.md)

## 1. Decision

Implement [Option C](0204-option-c-ro-base-rw-disk.md), the read-only
deterministic base plus a per-VM writable disk, as the storage design for
snapshot and restore, without first implementing Option B.

The [options survey](0204-root-volume-options.md#12-recommendation) recommended
Option B first and Option C as a follow-on. That recommendation was
conditional: "choose Option C instead, or as the follow-on, when the fleet
controls its guest images and needs bases shared across hosts". Section 2
records that both conditions hold, and that the remaining argument for B, a
smaller first change that ships sooner, does not apply.

Option B is not built. It stays a considered design candidate that may be
revisited. Option A is not chosen; its precondition (hosts that cannot leave
dm-thin pools) does not hold.

Section 3 lists the changes this decision makes to the Option C design. The
body of the Option C document is not revised here; that is a follow-up change.

## 2. Why the recommendation changed

The survey's recommendation for B rested on four properties: smallest change,
no guest image change, works on both VMMs, and containerd keeps provisioning.
The answers below remove the weight from the first two and leave C ahead on
the criteria that matter for a fleet.

| Question | Answer | Effect |
| -------- | ------ | ------ |
| Who controls guest images? | Users bring their own OCI images, but the project defines how images must be built. With decision D1 below no rootfs change is needed at all: the overlay init lives in an initrd that flintlock provides. | C's largest cost in the survey (an init in every image, [Option C §3](0204-option-c-ro-base-rw-disk.md#3-host-and-image-contract), §11, §12) is removed. |
| What is the topology? | A fleet of flintlock hosts, with snapshots taken on any host and restored on any host. | C's one base per image for every host beats A's and B's one base per host and image ([survey §10](0204-root-volume-options.md#10-comparison), "Base portability"). |
| Can hosts be reprovisioned onto a reflink filesystem? | Yes. | B and C are both available; A's only advantage (no host change) is not needed. |
| Must VMs created before this feature be snapshottable? | No. Snapshot support is opt-in at VM creation. | Removes the one case only A can serve. |
| Which containerd? | The move to containerd 2.x is already in progress. | C's containerd 2.2 dependency is no longer a cost; the base builder chosen in D8 does not need it anyway. |
| Is there a delivery deadline? | No. | B's ship-sooner advantage carries no weight. |
| Would B survive as a supported mode after C? | Undecided. | Building B first risks either two supported storage designs or throwaway work. Going to C directly avoids both. |

## 3. Design decisions and deltas to Option C

Each row names the section of the Option C document it amends. Where the row
says "new", Option C did not cover the topic.

| # | Topic | Decision | Amends |
| - | ----- | -------- | ------ |
| D1 | Overlay init location | The overlay init is shipped in an initrd that flintlock provides, not in the rootfs image. The initrd mounts the EROFS base, mounts the ext4 writable disk, assembles the overlayfs root, `pivot_root`s and execs the image's own init. The image label `dev.liquidmetal.flintlock/overlay-init` and the image rebuild requirement are dropped. Rootfs images are unchanged. | §3 (guest image), §5.4 (image label), §5.5 (kernel args), §7 (guest init failure row), §11 (guest images), §12 |
| D2 | Initrd selection | In overlay mode flintlock injects a default initrd image reference taken from daemon configuration when the spec omits `initrd`. A spec that names an initrd keeps it; that initrd is the user's responsibility and must honour the documented contract: the drive order flintlock emits and the kernel parameters it passes. | new |
| D3 | Initrd build location | The initrd is built in this repository under `hack/`, so it is versioned with `flintlockd`, whose drive order and kernel parameters it must understand. Guest kernel configuration changes go to `liquidmetal-dev/image-builder`, which builds the published kernel images. | new |
| D4 | Cloud Hypervisor initrd | The Cloud Hypervisor driver passes only `--kernel` and `--cmdline` today (`infrastructure/microvm/cloudhypervisor/create.go`) and never an initramfs; it must gain `--initramfs` for overlay mode. Firecracker already passes `initrd_path`. | §5.5 |
| D5 | Guest kernel requirement | Overlay mode requires a guest kernel of 5.10 or later built with `CONFIG_EROFS_FS` (plus the compression option used by D8) and `CONFIG_OVERLAY_FS`. EROFS is not available in 4.19, so the 4.19 kernel image stays usable in `full` mode only. The configuration of the published `flintlock-kernel:5.10.77` image has not been verified for EROFS; verifying it, or rebuilding it, in image-builder is a prerequisite (section 4). | §3 (guest kernel) |
| D6 | `mode: full` | `full` remains and is backed by the existing devmapper provisioning. It is not snapshottable and is deprecated. The default mode stays `full` for one release while the initrd and kernel artefacts are established, then flips to `overlay`. Option B is not built, so `full` is never backed by blockfile. | §5.4 |
| D7 | Thin pool optional | The provisioner makes the dm-thin pool optional. A host without one is overlay-only and rejects `full` VMs with a capability error, using the existing provider capability mechanism. | §5.5 (provisioning) |
| D8 | Base builder | Flintlock builds the base itself: `View` the image with the native snapshotter to get a directory tree, then one `mkfs.erofs` run with pinned `-T`, `-U` (derived from the OCI image digest), `--sort` and compression options, producing a single blob. The containerd erofs snapshotter route (§5.2) is not used; it can be revisited if unpack time for large images becomes a problem. | §3 (base construction), §5.1, §5.2, §12 (open question resolved) |
| D9 | Base format | EROFS. ext4 via `mke2fs -d` remains the fallback only if a kernel without EROFS must be supported, which D5 rules out for overlay mode. | §3 |
| D10 | Base distribution | Pull-first. The first host to use an image builds the base and pushes it. Every other host pulls the base by the digest recorded in the snapshot package and builds only when no base repository is configured. erofs-utils is still pinned in provisioning so that the degraded, build-locally path is reproducible. | §5.1, §8 (distribution), §12 (reproducibility) |
| D11 | Base repository | One daemon-configured base repository, keyed by OCI image digest. Bases are not pushed as referrers of the user's image manifest, because the fleet often has read-only access to user image repositories. The package records the qualified reference of the pushed base as well as its digest. | §8 (distribution) |
| D12 | Additional volumes | The overlay applies to the root volume only. A read-only additional volume becomes a single EROFS base drive. A writable additional volume becomes a deterministic ext4 image built from the OCI image with `mke2fs -d`, a fixed UUID and hash seed, reflinked per VM and captured whole on snapshot; the package carries it as a full block image, which SNAP-VOL-009 permits. A delta for these volumes can be added later. | §1, §5 ("each volume becomes two drives"), §9 |
| D13 | Writable disk size | Honour `Volume.Size` from the API (carried but unused today, `infrastructure/grpc/convert.go`), with a default from daemon configuration. Disks are sparse. One template per distinct size, created lazily. | §5.3 |
| D14 | Device lettering | The cloud-init disk-mount step derives guest device letters from the drive order the provider emits, as Option C proposes. This also fixes an existing Cloud Hypervisor mismatch: `core/steps/cloudinit/disk_mount.go` assigns `vdb` to the first additional volume, but the Cloud Hypervisor driver places the cloud-init image second, at `vdb`. | §5.4, §7 |
| D15 | Option B | Left as a considered design candidate that may be revisited. Its document is unchanged apart from its status line. | survey §12 |

## 4. Prerequisites outside this repository

* Verify that the published 5.10 guest kernel enables `CONFIG_EROFS_FS`, the
  compression chosen in D8, and `CONFIG_OVERLAY_FS`. If not, rebuild it in
  `liquidmetal-dev/image-builder` and publish a new tag. Overlay mode cannot be
  tested end to end until this exists.

## 5. Open for review

The numbered decisions D1 to D15 are the review checklist for this record.
Reviewers are asked to confirm or challenge each one; D1, D6, D10 and D12 have
the widest consequences.

## 6. Next steps

* On acceptance, capture the decision as an ADR in `docs/adr/`, as the
  [proposals index](../README.md) requires.
* Revise the body of the Option C document to incorporate D1 to D14 in a
  separate change.
* Update the requirements document's capture-options table if any SNAP-VOL
  wording is affected by D12.
