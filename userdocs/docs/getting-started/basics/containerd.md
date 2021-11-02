---
sidebar_position: 2
---

# Containerd

[Install ContainerD](https://github.com/containerd/containerd/releases).

_RunC is not required; Flintlock only uses the snapshotter._

## Create thinpool

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

## Configuration

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

## Start containerd

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
