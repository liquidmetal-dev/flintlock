# Flintlock scripts

This directory contains a number of useful scripts for both developing and running
flintlock.

## provision.sh

The `provision.sh` script can be used to bootstrap a production or development
ready host. The script includes commands to perform isolated setup steps.

Installation:

```bash
# if you have cloned the repository
./hack/scripts/provision.sh --help

# if you have not
wget https://raw.githubusercontent.com/liquidmetal-dev/flintlock/main/hack/scripts/provision.sh
chmod +x provision.sh

./provision.sh --help
```

To override component versions (default: `latest`):
```bash
export FLINTLOCK=
export FIRECRACKER=
export CONTAINERD=
```

Available commands:
```
usage: ./hack/scripts/provision.sh <COMMAND> <OPTIONS>

Script to provision hosts for running flintlock microvms

COMMANDS:

  all                    Complete setup for production ready host. Component versions
                         can be configured by setting the FLINTLOCK, CONTAINERD and FIRECRACKER
                         environment variables.
    OPTIONS:
      -y                 Autoapprove all prompts (danger)
      --skip-apt, -s     Skip installation of apt packages
      --thinpool, -t     Name of thinpool to create (default: flintlock or flintlock-dev)
      --disk, -d         Name blank unpartioned disk to use for direct lvm thinpool (ignored if --dev set)
      --grpc-address, -a Address on which to start the Flintlock GRPC server (default: local ipv4 address)
      --parent-iface, -i Interface of the default route of the host
      --dev              Set up development environment. Loop thinpools will be created.

  apt                    Install all apt packages required by flintlock

  firecracker            Install firecracker from feature branch
    OPTIONS:
      --version, -v      Version to install (default: latest)

  containerd             Install, configure and start containerd service
    OPTIONS:
      --version, -v      Version to install (default: latest)
      --thinpool, -t     Name of thinpool to include in config toml (default: flintlock-thinpool)
      --dev              Set up development environment. Containerd will keep state under 'dev' tagged paths.

  flintlock              Install and start flintlockd service (note: will not succeed without containerd)
    OPTIONS:
      --version, -v      Version to install (default: latest)
      --grpc-address, -a Address on which to start the GRPC server (default: local ipv4 address)
      --parent-iface, -i Interface of the default route of the host
      --dev              Assumes containerd has been provisioned in a dev environment

  direct_lvm             Set up direct_lvm thinpool
    OPTIONS:
      -y                 Autoapprove all prompts (danger)
      --thinpool, -t     Name of thinpool to create (default: flintlock)
      --disk, -d         Name blank unpartioned disk to use for direct lvm thinpool
      --skip-apt, -s     Skip installation of apt packages

  devpool                Set up loop device thinpool (development environments)
    OPTIONS:
      --thinpool, -t     Name of thinpool to create (default: flintlock-dev)
```

### all

The `all` subcommand will perform all the other subcommands and fully provision the
host. The default environment for this tool is production. The tool will format a block
device to act as a thinpool for containerd's devmapper snapshotter.

> Note: It is HIGHLY RECOMMENDED that users supply the name of a block device
they know for a fact to be unused via the `--disk` flag. The tool will make a
very naive attempt to find a disk which is not in use, but it should not be relied
upon in a production setting as all the data will be wiped from the selected device.

If you don't want or need to use production-ready thinpools (ie for development
environments), pass the `--dev` flag. This will provision a loop-backed thinpool,
therefore no physical device is required.

The provision script will default to binding the service address to the local IPv4 address.
For development purposes, or if you would prefer to access flintlockd from another
network, you can set `--grpc-address 0.0.0.0`.

### Other commands

Each step in the `all` provision command can be executed and configured
independently.

For example, `./provision.sh firecracker` will install the latest release of
the macvtap branch of firecracker.

## build-overlay-initrd.sh

The `build-overlay-initrd.sh` script builds `flintlock-overlay-init` and
packs it into a gzip compressed cpio archive with the binary at `/init`.
That archive is the initrd flintlock boots an overlay root volume with.
See `internal/overlayinit` for the guest contract.

```bash
# host architecture, written to out/
make build-overlay-initrd

# amd64 and arm64
make build-release-overlay-initrd

# or directly
./hack/scripts/build-overlay-initrd.sh out/flintlock-overlay-initrd_amd64.img amd64
```

The release workflow publishes the archives as the multi-arch image
`ghcr.io/liquidmetal-dev/flintlock-overlay-init:<version>` with the file at
`/initrd.img` (`hack/packaging/flintlock-overlay-init.Dockerfile`).

## build_erofs_utils.sh

Builds the pinned static `mkfs.erofs` that `flintlock-provision erofs-utils`
installs. CI runs it for every release and for the test job. Run it yourself
to get the same binary:

```bash
# needs git, build-essential, autoconf, automake, libtool and pkg-config
./hack/scripts/build_erofs_utils.sh 1.9.4 ./bin/erofs-utils

# or in a linux/amd64 container, for example on an arm64 host
./hack/scripts/build_erofs_utils.sh --docker 1.9.4 ./bin/erofs-utils
```

The output is `mkfs.erofs-<version>-linux-amd64` and its `.sha256` file.
