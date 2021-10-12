# Getting started with reignite

## Configure network

If you are using wired connection, you can skip this and jump straight to the
"Containerd" section. With wireless adapter, macvtap has some issues. The easy
workaround is to use a bridge and tap devices instead.

You can use the default kvm network, in this case, skip to "Create and connect
tap device" and use `default`. We recommend using a dedicated network to avoid
interference from other kvm machines or processes like IP or MAC address
conflict.

### Create kvm network

Create the `reignite.xml` file (feel free to change the IP range):

```xml
<network>
  <name>reignite</name>
  <forward mode='nat'>
    <nat>
      <port start='1024' end='65535'/>
    </nat>
  </forward>
  <bridge name='rgntbr0' stp='on' delay='0'/>
  <ip address='192.168.100.1' netmask='255.255.255.0'>
    <dhcp>
      <range start='192.168.100.2' end='192.168.100.254'/>
    </dhcp>
  </ip>
</network>
```

Define, start and set autostart on the `reignite` network:

```
virsh net-define reignite.xml
virsh net-start reignite
virsh net-autostart reignite
```

Now you should see the network in the network list:

```
virsh net-list
 Name       State    Autostart   Persistent
---------------------------------------------
 default    active   yes         yes
 reignite   active   yes         yes
```

### Create and connect tap device

```bash
tapName=tap0
bridge=rgntbr0
sudo ip tuntap add ${tapName} mode tap
sudo ip link set ${tapName} master ${bridge} up
```

You can add a function into your bashrc/zshrc:

```bash
function vir-new-tap() {
  tapName=${1:=tap0}
  bridge=${2:=rgntbr0}

  sudo ip tuntap add ${tapName} mode tap
  sudo ip link set ${tapName} master ${bridge} up
}
```

You can check the DHCP leases with `virsh`:

```bash
virsh net-dhcp-leases default
```

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

```bash
# Just to make sure all the directories are there.
sudo mkdir -p /var/lib/containerd-dev/snapshotter/devmapper
sudo mkdir -p /run/containerd-dev/

sudo containerd --config /etc/containerd/config-dev.toml
```

To reach our new dev containerd, we have to specify the `--address` flag,
for example:

```bash
sudo ctr \
    --address=/run/containerd-dev/containerd.sock \
    --namespace=reignite \
    content ls
```

To make it easier, here is an alias:

```bash
alias ctr-dev="sudo ctr --address=/run/containerd-dev/containerd.sock"
```

## Set up Firecracker

We have to use a custom built firecracker from the macvtap branch
([see][discussion-107]).

```bash
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

```bash
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

```bash
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
