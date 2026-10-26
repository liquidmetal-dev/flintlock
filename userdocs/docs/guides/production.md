---
title: Production
---

`flintlock-provision` prepares a host to run `flintlockd`. It installs
Firecracker, Cloud Hypervisor, containerd and `flintlockd`, and it sets up
the storage each boot mode needs.

Download the binary from a [release](https://github.com/liquidmetal-dev/flintlock/releases)
or build it with `make build`. Run every command as root.

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

## Other commands

Each step of `all` is also a command of its own: `apt`, `firecracker`,
`cloudhypervisor`, `containerd`, `flintlock`, `direct-lvm` and `devpool`.
`containerd --skip-thinpool` writes a configuration without the devmapper
section. Run `flintlock-provision --help` for the full list of flags.

To pin a component version, set the `FLINTLOCK`, `CONTAINERD`, `FIRECRACKER`
or `CLOUD_HYPERVISOR` environment variable before you run the tool.
