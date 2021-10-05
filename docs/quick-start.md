# Getting started with reignite

## Set up networking

```
# Default network device.
NET_DEVICE=$(ip route show | awk '/default/ {print $5}')

# Create a macvtap device.
sudo ip link add link "${NET_DEVICE}" name macvtap0 type macvtap

# Static MAC address can be set,
# otherwise it gets a random auto-generated address.
sudo ip link set macvtap0 address 1a:46:0b:ca:bc:7b up

# Check the MAC address of the device.
ip link show macvtap0

# You can read the MAC address always under /sys without parsing
# the output of `ip link show`.
cat /sys/class/net/macvtap0/address
```

## Containerd

### Create thinpool

Easy quick-start option is to run this script as root. I know,
it's not recommended in general, and I'm happy you think it's not a good
way to do things, so there are comments for commands.

```bash
#!/bin/bash

set -ex

if [[ $(id -u) != 0 ]]; then
  echo "Run this script as root..." >&2
  exit 1
fi

# That's where our stuff will live.
CROOT=/var/lib/containerd-dev
# This is the name of the thinpool.
POOL=dev-thinpool

mkdir -p "${CROOT}/snapshotter/devmapper"

DIR="${CROOT}/snapshotter/devmapper"

# Create "data" file/volume if it's not there and set it's size to 100G.
if [[ ! -f "${DIR}/data" ]]; then
touch "${DIR}/data"
truncate -s 100G "${DIR}/data"
fi

# Create "metadata" file/volume if it's not there and set it's size to 2G.
if [[ ! -f "${DIR}/metadata" ]]; then
touch "${DIR}/metadata"
truncate -s 10G "${DIR}/metadata"
fi

# Find/associate a loop device with our data volume.
DATADEV="$(sudo losetup --output NAME --noheadings --associated ${DIR}/data)"
if [[ -z "${DATADEV}" ]]; then
    DATADEV="$(sudo losetup --find --show ${DIR}/data)"
fi

# Find/associate a loop device with our metadata volume.
METADEV="$(sudo losetup --output NAME --noheadings --associated ${DIR}/metadata)"
if [[ -z "${METADEV}" ]]; then
    METADEV="$(sudo losetup --find --show ${DIR}/metadata)"
fi

# Magic calculations, for more information go and read
# https://www.kernel.org/doc/Documentation/device-mapper/thin-provisioning.txt
SECTORSIZE=512
DATASIZE="$(blockdev --getsize64 -q ${DATADEV})"
LENGTH_SECTORS=$(bc <<< "${DATASIZE}/${SECTORSIZE}")
DATA_BLOCK_SIZE=128
# picked arbitrarily
# If free space on the data device drops below this level then a dm event will
# be triggered which a userspace daemon should catch allowing it to extend the
# pool device.
LOW_WATER_MARK=32768

THINP_TABLE="0 ${LENGTH_SECTORS} thin-pool ${METADEV} ${DATADEV} ${DATA_BLOCK_SIZE} ${LOW_WATER_MARK} 1 skip_block_zeroing"
echo "${THINP_TABLE}"

# If thinpool does not exist yet, create one.
if ! $(dmsetup reload "${POOL}" --table "${THINP_TABLE}"); then
    sudo dmsetup create "${POOL}" --table "${THINP_TABLE}"
fi

cat << EOF
#
# Add this to your config.toml configuration file and restart containerd daemon
#
[plugins]
  [plugins.devmapper]
    pool_name = "${POOL}"
    root_path = "${DIR}"
    base_image_size = "10GB"
    discard_blocks = true
EOF
```

I hope all my comments are enough to understand what this script does.

### Configuration

```toml
# /etc/containerd/config-dev.toml
version = 2

root = "/var/lib/containerd-dev"
state = "/run/containerd-dev"

[grpc]
  address = "/run/containerd-dev/containerd.sock"

[metrics]
  address = "127.0.0.1:1338"

[plugins]
  [plugins."io.containerd.snapshotter.v1.devmapper"]
    pool_name = "dev-thinpool"
    root_path = "/var/lib/containerd-dev/snapshotter/devmapper"
    base_image_size = "10GB"
    discard_blocks = true

[debug]
  level = "trace"
```

### Start containerd

```
# Just to make sure all the directories are there.
sudo mkdir -p /var/lib/containerd-dev/snapshotter/devmapper
sudo mkdir -p /run/containerd-dev/

sudo containerd --config /etc/containerd/config-dev.toml
```

To reach our new dev containerd, we have to specify the `--address` flag,
for example:

```
sudo ctr \
    --address=/run/containerd-dev/containerd.sock \
    --namespace=reignite \
    content ls
```

To make it easier, here is an alias:

```
alias ctr-dev="sudo ctr --address=/run/containerd-dev/containerd.sock"
```

## Set up Firecracker

We have to use a custom built firecracker from the macvtap branch
([see][discussion-107]).

```
git clone https://github.com/firecracker-microvm/firecracker.git
git fetch origin feature/macvtap
git checkout -b feature/macvtap origin/feature/macvtap
# This will build it in a docker container, no rust installation required.
tools/devtool build

# Any directories on $PATH.
TARGET=~/local/bin
toolbox=$(uname -m)-unknown-linux-musl

cp build/cargo_target/${toolbox}/debug/{firecracker,jailer} ${TARGET}
```

If you don't have to compile it yourself, you can download a pre-built version
from the [Pre-requisities discussion][discussion-107].

[discussion-107]: https://github.com/weaveworks/reignite/discussions/107

## Set up and start reignite

```
go mod download
make build

NET_DEVICE=$(ip route show | awk '/default/ {print $5}')

./bin/reignited run \
  --containerd-socket=/run/containerd-dev/containerd.sock \
  --parent-iface="${NET_DEVICE}"
```

## BloomRPC

[BloomRPC][bloomrpc] is a good tool to test gRPC endpoint.

### Import

Use the "Import Paths" button and add `$repo/api` to the list. All available
endpoints will be visible in a nice tree view.

### Example

TODO: Example CreateVM Payload

[bloomrpc]: https://github.com/uw-labs/bloomrpc

## Troubleshooting

### Reignited fails to start with `failed to reconcile vmid`

Example error:

```
ERRO[0007] failed to reconcile vmid Hello/aa3b711d-4b60-4ba5-8069-0511c213308c: getting microvm spec for reconcile: getting vm spec from store: finding content in store: walking content store for aa3b711d-4b60-4ba5-8069-0511c213308c: context canceled  controller=microvm
```

There is a plan to create a VM, but something went wrong. The easiest way to
fix it to remove it from containerd:

```
vmid='aa3b711d-4b60-4ba5-8069-0511c213308c'
contentHash=$(\
  ctr-dev \
    --namespace=reignite \
    content ls \
    | awk "/${vmid}/ {print \$1}" \
)
ctr-dev \
    --namespace=reignite \
    content rm "${contentHash}"
```
