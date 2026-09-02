---
title: Containerd
---

Flintlock uses containerd to pull and snapshot OS and kernel images, as well as
to store MicroVM metadata.

## Devmapper

Flintlock relies on containerd's devicemapper snapshotter to provide filesystem
devices for Firecracker MicroVMs. Some configuration is required.

Flintlock supplies a general tool for provisioning hosts.

```bash
sudo apt update
sudo apt install -y dmsetup bc

sudo ./hack/scripts/provision.sh devpool
```

Verify with `sudo dmsetup ls`.

## Containerd
### Configure

It is likely that you will already have `containerd` running somewhere: it is
used by Docker.

For "tidiness" we will run a separate containerd process. The two will not interfere.

Write a new containerd configuration file.
Save this config to `/etc/containerd/config-dev.toml`.

```bash
cat << EOF >/etc/containerd/config-dev.toml
version = 2

root = "/var/lib/containerd-dev"
state = "/run/containerd-dev"

[grpc]
  address = "/run/containerd-dev/containerd.sock"

[metrics]
  address = "127.0.0.1:1338"

[plugins]
  [plugins."io.containerd.snapshotter.v1.devmapper"]
    pool_name = "flintlock-dev-thinpool"
    root_path = "/var/lib/containerd-dev/snapshotter/devmapper"
    base_image_size = "10GB"
    discard_blocks = true

[debug]
  level = "trace"
EOF
```

Create all the state and run directories:
```bash
sudo mkdir -p /var/lib/containerd-dev/snapshotter/devmapper
sudo mkdir -p /run/containerd-dev/
```

## Start

[Install ContainerD](https://github.com/containerd/containerd/releases).

_RunC is not required; Flintlock uses various containerd services only._

```bash
sudo containerd --config /etc/containerd/config-dev.toml
```

Containerd will log about 100 lines at boot, most will be about loading plugins, and we recommended
scrolling up to ensure that the devmapper plugin loaded successfully.

Towards the end you should see something like `containerd successfully booted in 0.055357s`.

To reach our new dev containerd, we have to specify the `--address` flag,
for example:

```bash
sudo ctr \
    --address=/run/containerd-dev/containerd.sock \
    --namespace=flintlock \
    content ls
```

:::tip
To make it easier, save the command to an alias:

```bash
alias ctr-dev="sudo ctr --address=/run/containerd-dev/containerd.sock"
```
:::

You can either background the `containerd` process or open another shell window.

## Private registry authentication

Flintlock's containerd client does not configure any registry credentials
itself. By default, all image pulls rely entirely on how the containerd
**daemon** you point flintlock at (via `--containerd-socket`) is configured
for registry access.

If you need to pull images from a private registry without editing the
daemon's global config, start flintlock with `--containerd-hosts-dir`
pointing at a directory laid out using containerd's standard
[hosts.toml](https://containerd.io/docs/hosts/) (`certs.d`) format:

```
<hosts-dir>/
  <registry-host>/
    hosts.toml
```

For example, to configure auth for `registry.example.com`:

```bash
sudo mkdir -p /etc/flintlock/certs.d/registry.example.com
cat << EOF >/etc/flintlock/certs.d/registry.example.com/hosts.toml
server = "https://registry.example.com"

[host."https://registry.example.com"]
  [host."https://registry.example.com".header]
    Authorization = "Basic <base64-encoded-user:password>"
EOF

flintlockd run --containerd-hosts-dir /etc/flintlock/certs.d ...
```

When `--containerd-hosts-dir` is left unset (the default), flintlock's image
pulls behave exactly as before, with no per-registry configuration of their
own.
