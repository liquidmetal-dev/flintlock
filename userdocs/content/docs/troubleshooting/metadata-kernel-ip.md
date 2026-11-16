---
weight: 3
title: Firecracker metadata interface
---

When a Firecracker MicroVM has `add_network_config` set, flintlock
configures its metadata interface (`eth0` in the guest) with the kernel
`ip=` parameter, for example:

```
ip=169.254.0.1:::255.255.0.0:localhost:eth0:off
```

The metadata interface is then not in the `network-config`, so
systemd-networkd does not manage it and does not run IPv4 address
conflict detection on it. That check made each boot wait about 6 s, two
times. See [ADR 1396][adr] for the reasons.

## The guest gets no meta-data or user-data

cloud-init does not run your user-data, and in the guest `eth0` has no
IPv4 address.

The guest kernel does not have `CONFIG_IP_PNP`, so it ignores the `ip=`
parameter. The liquidmetal-dev Firecracker kernels have it.

### Fix

Either:

- Use a guest kernel with `CONFIG_IP_PNP=y`, or
- Run `flintlockd` with `--firecracker-metadata-kernel-ip=false`. The
  metadata interface is then in the `network-config` again, and the boot
  waits for the address conflict detection again.

## The boot still waits for `systemd-networkd-wait-online`

The guest image enables `systemd-network-generator.service`. It turns the
`ip=` parameter into a systemd-networkd configuration, so
systemd-networkd manages `eth0` and runs address conflict detection again.

### Fix

Disable `systemd-network-generator.service` in the guest image.

## Your kernel command line sets `ip`

If the MicroVM spec sets `ip` in `kernel.cmdline`, flintlock keeps your
value and writes the metadata interface into the `network-config` as
before.

[adr]: https://github.com/liquidmetal-dev/flintlock/blob/main/docs/adr/1396-metadata-interface-kernel-ip.md
