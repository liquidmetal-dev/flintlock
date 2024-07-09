---
title: Flintlock
---

Once you have installed `firecracker` and started `containerd`, you can start
the `flintlockd` service.

You can either download a [release][flint], or build locally:

```bash
go mod download
make build
```

Flintlock will create MicroVMs with interfaces tapped to a parent on the host.
If you have a wired connection (and you did not set up a bridge earlier), this will
be the ethernet interface. If you are on a wireless connection (and you did need to
create a bridge) this will be your wireless interface.

If you only have the one interface, this command will find it for you:

```bash
NET_DEVICE=$(ip route show | awk '/default/ {print $5}')
```

If you have both, you can use the above command (less the `print` bit) or `ip link show`,
`nmcli con show`, etc, and do it by eye.

```bash
NET_DEVICE=<your parent interface name>
```

Lastly we start `flintlockd` with the address to our `containerd`, and the `parent-iface`
name:

```bash
sudo ./bin/flintlockd run \
  --containerd-socket=/run/containerd-dev/containerd.sock \
  --parent-iface="${NET_DEVICE}" \
  --insecure
```

:::tip
If you're running `flintlockd` from within a Vagrant VM and wish to call the
gRPC API from your host machine then you need to run `flintlockd` with the
`--grpc-endpoint=0.0.0.0:9090` flag, otherwise the connection will be rejected.
:::

You should see it start successfully with similar output:

```
INFO[0000] flintlockd, version=undefined, built_on=undefined, commit=undefined
INFO[0000] flintlockd grpc api server starting
INFO[0000] starting microvm controller
INFO[0000] starting microvm controller with 1 workers    controller=microvm
INFO[0000] resyncing microvm specs                       controller=microvm
INFO[0000] Resyncing specs                               action=resync controller=microvm namespace=ns
WARN[0000] basic authentication is DISABLED
WARN[0000] TLS is DISABLED
INFO[0000] starting event listener                       controller=microvm
INFO[0000] Starting workersnum_workers1                  controller=microvm
```

[flint]: https://github.com/liquidmetal-dev/flintlock/releases
