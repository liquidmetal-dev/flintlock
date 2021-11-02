# Getting started with flintlock

<!--
To update the TOC, install https://github.com/kubernetes-sigs/mdtoc
and run: mdtoc -inplace docs/quick-start.md
-->

<!-- toc -->
- [MacOS Users](#macos-users)
- [Configure network](#configure-network)
  - [Create kvm network](#create-kvm-network)
  - [Create and connect tap device](#create-and-connect-tap-device)
- [Containerd](#containerd)
  - [Create thinpool](#create-thinpool)
  - [Configuration](#configuration)
  - [Start containerd](#start-containerd)
- [Set up Firecracker](#set-up-firecracker)
- [Set up and start flintlock](#set-up-and-start-flintlock)
- [Interacting with the service](#interacting-with-the-service)
  - [grpc-client-cli](#grpc-client-cli)
    - [Example](#example)
  - [BloomRPC](#bloomrpc)
    - [Import](#import)
    - [Example](#example-1)
- [Troubleshooting](#troubleshooting)
  - [flintlockd fails to start with <code>failed to reconcile vmid</code>](#flintlockd-fails-to-start-with-)
<!-- /toc -->

## MacOS Users

Flintlock and flintlock tests are only compatible with Linux. We recommend that
non-linux users provision a Linux VM in which to work.

You can use Vagrant:

```
vagrant up
```

It will create a new pre-configured machine ready to use.
Run the rest of the instructions on this page on that machine.

## Configure network

If you are using wired connection, you can skip this and jump straight to the
"Containerd" section. With wireless adapter, macvtap has some issues. The easy
workaround is to use a bridge and tap devices instead.

You can use the default kvm network, in this case, skip to "Create and connect
tap device" and use `default`. We recommend using a dedicated network to avoid
interference from other kvm machines or processes like IP or MAC address
conflict.

### Install packages and start `libvirtd`

```bash
sudo apt install qemu qemu-kvm libvirt-clients libvirt-daemon-system virtinst bridge-utils

sudo systemctl enable libvirtd
sudo systemctl start libvirtd
```

### Create kvm network

Create the `flintlock.xml` file (feel free to change the IP range):

```xml
<network>
  <name>flintlock</name>
  <forward mode='nat'>
    <nat>
      <port start='1024' end='65535'/>
    </nat>
  </forward>
  <bridge name='flkbr0' stp='on' delay='0'/>
  <ip address='192.168.100.1' netmask='255.255.255.0'>
    <dhcp>
      <range start='192.168.100.2' end='192.168.100.254'/>
    </dhcp>
  </ip>
</network>
```

Define, start and set autostart on the `flintlock` network:

```
sudo virsh net-define flintlock.xml
sudo virsh net-start flintlock
sudo virsh net-autostart flintlock
```

Now you should see the network in the network list:

```
virsh net-list
 Name       State    Autostart   Persistent
---------------------------------------------
 default    active   yes         yes
 flintlock   active   yes         yes
```

### Create and connect tap device

```bash
tapName=tap0
bridge=flkbr0
sudo ip tuntap add ${tapName} mode tap
sudo ip link set ${tapName} master ${bridge} up
```

Check with `ip link ls`.

You can add a function into your bashrc/zshrc:

```bash
function vir-new-tap() {
  tapName=${1:=tap0}
  bridge=${2:=flkbr0}

  sudo ip tuntap add ${tapName} mode tap
  sudo ip link set ${tapName} master ${bridge} up
}
```

You can check the DHCP leases with `virsh`:

```bash
sudo virsh net-dhcp-leases default
```

## Containerd

[Install ContainerD](https://github.com/containerd/containerd/releases).

_RunC is not required; Flintlock only uses the snapshotter._

### Create thinpool

Flintlock relies on ContainerD's devicemapper snapshotter to provide filesystem
devices for Firecracker microvms. Some configuration is required.

The easy quick-start option is to run the `hack/scripts/devpool.sh` script as root.
I know, it's not recommended in general, and I'm happy you think it's not a good
way to do things, read the comments in the script for details.

```bash
sudo apt update
sudo apt install -y dmsetup bc

sudo ./hack/scripts/devpool.sh
```

Verify with `sudo dmsetup ls`.

### Configuration

Save this config to `/etc/containerd/config-dev.toml`.

```toml
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

containerd will log about 100 lines at boot, most will be about loading plugins, and we recommended
scrolling up to ensure that the devmapper plugin loaded successfully.

Towards the end you should see `containerd successfully booted in 0.055357s`.

To reach our new dev containerd, we have to specify the `--address` flag,
for example:

```bash
sudo ctr \
    --address=/run/containerd-dev/containerd.sock \
    --namespace=flintlock \
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

[discussion-107]: https://github.com/weaveworks/flintlock/discussions/107

## Set up and start flintlock

```bash
go mod download
make build

NET_DEVICE=$(ip route show | awk '/default/ {print $5}')

sudo ./bin/flintlockd run \
  --containerd-socket=/run/containerd-dev/containerd.sock \
  --parent-iface="${NET_DEVICE}"
```

If you're running `flintlockd` from within a Vagrant VM and wish to call the gRPC API from your host machine then you need to run `flintlockd` with the `--grpc-endpoint=0.0.0.0:9090` flag, otherwise the connection will be rejected.

You should see it start successfully with similar output:
```
INFO[0000] flintlockd, version=undefined, built_on=undefined, commit=undefined
INFO[0000] flintlockd grpc api server starting
INFO[0000] starting microvm controller
INFO[0000] starting microvm controller with 1 workers    controller=microvm
INFO[0000] resyncing microvm specs                       controller=microvm
INFO[0000] Resyncing specs                               action=resync controller=microvm namespace=ns
INFO[0000] starting event listener                       controller=microvm
INFO[0000] Starting workersnum_workers1                  controller=microvm
```

## Interacting with the service

We recommend using one of the following tools to send requests to the Flintlock server.
There are both GUI and a CLI option.

### grpc-client-cli

Install the [grpcurl][grpcurl].

Use the `./hack/scripts/send.sh` script.

#### Example

To created a MicroVM:

```
./hack/scripts/send.sh \
  --method CreateMicroVM
```

In the terminal where you started the Flintlock server, you should see in the logs that the MircoVM
has started.

### BloomRPC

[BloomRPC][bloomrpc] is a GUI tool to test gRPC endpoints.

#### Import

To import Flintlock protos into the Bloom GUI:

1. Click `Import Paths` on the left-hand menu bar and add `<absolute-repo-path>/api` to the list
1. Click the import `+` button and select `flintlock/api/services/microvm/v1alpha1/microvms.proto`

All available endpoints will be visible in a nice tree view.

#### Example

To create a MircoVM, select the `CreateMicroVM` endpoint in the left-hand menu.
Replace the sample request JSON in the left editor panel with [this example](hack/scripts/payload/CreateMicroVM.json).
Click the green `>` in the centre of the screen. The response should come immediately.

In the terminal where you started the Flintlock server, you should see in the logs that the MircoVM
has started.

To delete the MircoVM, select the `DeleteMicroVM` endpoint in the left-hand menu.
Replace the sample request JSON in the left editor panel with [this example](hack/scripts/payload/DeleteMicroVM.json).
Take care to ensure the values match those of the MicroVM you created earlier.
Click the green `>` in the centre of the screen. The response should come immediately.

**Note: there are example payloads for other endpoints, but not all are implemented at present.**

[grpcurl]: https://github.com/fullstorydev/grpcurl
[bloomrpc]: https://github.com/uw-labs/bloomrpc

## Troubleshooting

### flintlockd fails to start with `failed to reconcile vmid`

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
    --namespace=flintlock \
    content ls \
    | awk "/${vmid}/ {print \$1}" \
)
ctr-dev \
    --namespace=flintlock \
    content rm "${contentHash}"
```
