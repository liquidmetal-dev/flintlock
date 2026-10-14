---
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
in that case. A `mkfs.erofs` from the distribution is not touched.

The asset is fetched from the flintlock release of the same version as
`flintlock-provision` itself. `--version` or the `FLINTLOCK` environment
variable select another release. Releases older than the one that added
the asset do not carry it, and the install fails with a 404.

`flintlockd` looks for the binary at the install path. Use
`--mkfs-erofs-path` to point it somewhere else, for example at a build you
made yourself with `hack/scripts/build_erofs_utils.sh`.

[releases]: https://github.com/liquidmetal-dev/flintlock/releases
