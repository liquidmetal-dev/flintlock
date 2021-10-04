# Getting started with reignite

## Set up networking

```
# Default network device
NET_DEVICE=$(ip route show | awk '/default/ {print $5}')

# Create a tap device
sudo ip tuntap add tap0 mode tap
sudo ip addr add 172.100.0.1/24 dev tap0
sudo ip link set tap0 up

# Forward tap to default device
sudo iptables -A FORWARD -i tap0 -o $NET_DEVICE -j ACCEPT

# MAC address for tap0
# Can be useful later
export TAP0_MAC="$(cat /sys/class/net/tap0/address)"
```

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
truncate -s 2G "${DIR}/metadata"
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
  [plugins."io.containerd.grpc.v1.cri".containerd]
    snapshotter = "devmapper"
  [plugins."io.containerd.snapshotter.v1.devmapper"]
    pool_name = "dev-thinpool"
    base_image_size = "10GB"
    root_path = "/var/lib/containerd-dev/snapshotter/devmapper"

[debug]
  level = "debug"
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

## Start Firecracker

```
rm /tmp/firecracker.socket && \
  firecracker --api-sock /tmp/firecracker.socket
```

## Set up and start reignite

```
go mod download
make build

./bin/reignited run \
  --containerd-socket=/run/containerd-dev/containerd.sock \
  --firecracker-api=/tmp/firecracker.socket \
  --parent-iface=tap0
```

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
