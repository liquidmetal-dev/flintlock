---
weight: 7
title: Provisioning a host
---

`flintlock-provision` sets up a host to run flintlock. Each subcommand does
one step. `all` runs them in order for a production host.

Download the binary from the [releases page][releases], or build it with
`make build`.

```bash
flintlock-provision --help
```

| Subcommand | What it does |
| ---------- | ------------ |
| `all` | Complete setup: apt packages, Firecracker, Cloud Hypervisor, containerd, flintlockd and a thinpool |
| `apt` | Install the apt packages flintlock needs |
| `firecracker` | Install Firecracker to `/usr/local/bin` |
| `cloudhypervisor` | Install Cloud Hypervisor to `/usr/local/bin` |
| `containerd` | Install, configure and start containerd |
| `flintlock` | Install, configure and start flintlockd |
| `erofs-utils` | Install the pinned `mkfs.erofs` used to build overlay base images |
| `state-dir` | Check or create the reflink state directory that overlay roots need |
| `direct-lvm` | Create a direct-lvm thinpool on a blank disk |
| `devpool` | Create a loop-backed thinpool for development |

Set the `FLINTLOCK`, `CONTAINERD`, `FIRECRACKER` and `CLOUD_HYPERVISOR`
environment variables to pin component versions. The default is the latest
release.

## erofs-utils

Overlay boot builds the base image of a microVM with `mkfs.erofs`. The build
must give the same output on every host, so flintlock pins the exact
erofs-utils version. Distribution packages are too old: Ubuntu 24.04 ships
1.7.1, and the build needs 1.9.4 or newer.

Every flintlock release carries a static `mkfs.erofs` for `linux/amd64`,
built in CI from the erofs-utils git tag, together with its sha256:

```text
mkfs.erofs-<erofs-utils version>-linux-amd64
mkfs.erofs-<erofs-utils version>-linux-amd64.sha256
```

`flintlock-provision erofs-utils` downloads both, checks the sha256 and
installs the binary at `/opt/flintlock/bin/mkfs.erofs`:

```bash
sudo flintlock-provision erofs-utils
```

A wrong checksum stops the install. Nothing is written to the install path
in that case. A `mkfs.erofs` from the distribution is not touched. The release
ships the binary for `linux/amd64` only; on any other host the command stops
before it downloads anything ([#1317][arm64] tracks arm64).

The asset is fetched from the flintlock release of the same version as
`flintlock-provision` itself. `--version` or the `FLINTLOCK` environment
variable select another release. Releases older than the one that added
the asset do not carry it, and the install fails with a 404.

`flintlockd` looks for the binary at the install path. Use
`--mkfs-erofs-path` to point it somewhere else, for example at a build you
made yourself with `hack/scripts/build_erofs_utils.sh`.

## Storage a host needs

Flintlock has two ways to give a MicroVM its root disk.

- A `full` root volume is a device from a containerd thin pool. The
  devmapper snapshotter needs the pool.
- An `overlay` root is a read-only base image plus a small writable disk.
  The writable disk is a reflink of a template in the flintlockd state
  directory. The state directory must be on a filesystem that supports the
  `FICLONE` ioctl: XFS with `reflink=1` or Btrfs.

Kernel and initrd images use the native snapshotter. They pull on a host
with no thin pool.

The host advertises the modes its storage supports. A host is in one of
three states:

| Host has | Advertises | Accepts |
| -------- | ---------- | ------- |
| Thin pool only | `volume-full` | `full` roots and `full` additional volumes |
| Thin pool and reflink state directory | `volume-full` and `volume-overlay` | both root modes, and overlay roots with `full` additional volumes |
| Reflink state directory only | `volume-overlay` | overlay roots with no container additional volumes |

## `all`

`all` does every step. By default it formats a block device as a thin pool
for containerd.

```bash
flintlock-provision all --disk /dev/sdb --grpc-address 0.0.0.0
```

Give `--disk` the name of a device you know is unused. Without the flag the
tool looks for a free disk on its own. Do not rely on that on a production
host. All data on the chosen device is lost.

Pass `--dev` for a development host. It creates a loop-backed thin pool, so
no spare device is needed.

Pass `--skip-thinpool` for a host that serves overlay roots only. The tool
then creates no pool and leaves the devmapper section out of the containerd
configuration. containerd starts with its native snapshotter. Run
`state-dir` as well, so the host has a reflink state directory.

```bash
flintlock-provision all --skip-thinpool
flintlock-provision state-dir --device /dev/sdb
```

## `state-dir`

`state-dir` checks or creates the flintlockd state directory. The default
directory is `/var/lib/flintlock`. Pass `--dir` to use another one. It must
match the `--state-dir` flag of `flintlockd`.

With no other flags the command only verifies. It creates a small file in
the directory, clones it with `FICLONE` and removes both. It changes
nothing else. The command fails with a clear error when the directory is on
a filesystem without reflink support, for example ext4 or XFS formatted
without `reflink=1`.

```bash
flintlock-provision state-dir
```

Pass `--device` to format a block device or logical volume. The tool
formats it as XFS with `reflink=1`, mounts it at the state directory and
writes a line to `/etc/fstab` by UUID, so the mount comes back after a
reboot. The tool refuses a device that already holds a filesystem or a
partition table. Pass `--force` to format it anyway.

```bash
flintlock-provision state-dir --device /dev/sdb
```

Pass `--loop-file` for a development or e2e host with no spare device. The
tool creates a sparse file of `--size` GB (default 20), formats it as XFS
with `reflink=1` through a loop device, and mounts it at the state
directory. The `/etc/fstab` line uses the `loop` and `nofail` options, so
the mount comes back after a reboot and a missing file does not block the
boot.

```bash
flintlock-provision state-dir --loop-file /var/lib/flintlock-state.img --size 20
```

Both forms ask for confirmation before they format anything. Pass `-y` to
skip the prompt.

`state-dir` works on a fresh host or on a host with no MicroVMs. It does not
move an existing state directory with MicroVMs on it.

[releases]: https://github.com/liquidmetal-dev/flintlock/releases
[arm64]: https://github.com/liquidmetal-dev/flintlock/issues/1317
