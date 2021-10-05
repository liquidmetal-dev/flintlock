# Getting started with reignite

## Containerd

### Create thinpool

Easy quick-start option is to run the `hack/scripts/devpool.sh` script as root.
I know, it's not recommended in general, and I'm happy you think it's not a good
way to do things, read the comments in the script for details.

```bash
sudo ./hack/scripts/devpool.sh
```

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
