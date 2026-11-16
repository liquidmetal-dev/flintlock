# 1396. The guest kernel configures the metadata interface

- Status: accepted
- Date: 2026-11-16
- Authors: @richardcase
- Deciders: @richardcase
- ADR Discussion: https://github.com/liquidmetal-dev/flintlock/issues/1396

## Context

flintlock gives the metadata interface of a Firecracker microVM the address `169.254.0.1/16` (ADR 0241), and writes it into the netplan network-config. For an IPv4 link-local address, systemd-networkd does address conflict detection (ACD) before it uses the address. In the guest, ACD occurs two times in each boot and takes about 6 s each time. `network-online.target` and cloud-init wait for it. No other host is on this link, so the check gives no value.

Firecracker MMDS accepts only a link-local address. netplan has no setting to disable ACD and no way to mark an interface unmanaged. systemd 255 cannot read networkd configuration from credentials. Address autodetection in the guest, for example Calico `first-found`, skips 169.254.0.0/16 addresses, so the address must stay link-local.

## Decision

When flintlock adds the network-config for a Firecracker microVM, it leaves the metadata interface out of the netplan. It configures the interface with the kernel `ip=` parameter, for example `ip=169.254.0.1:::255.255.0.0:localhost:eth0:off`. The kernel sets the address without a conflict check, and systemd-networkd does not manage the interface. The hostname field is `localhost`, because the kernel otherwise sets the hostname to the address.

The flintlockd flag `--firecracker-metadata-kernel-ip` turns this on. It is `false` by default, so a guest kernel without `CONFIG_IP_PNP` does not lose its metadata on upgrade. When the flag is `false`, or when the spec kernel cmdline already has `ip`, flintlock writes the metadata interface into the netplan as before. After operators have had notice in the release notes, a future release will change the default to `true`.

Boot tests with Firecracker v1.18.0, the liquidmetal 5.10 kernel and the Ubuntu 24.04 image: the wait for `network-online.target` went from about 6 s to 0.02 s, and the time to the end of cloud-init went down by about 10.5 s. MMDS answered on the first boot, on a reboot, and on a restart of a microVM that was first booted with the old configuration.

## Considered options

- **Address `192.0.0.8/32` with an on-link route to `169.254.169.254/32`.** This has no ACD, but a global address on the first interface is picked by address autodetection such as Calico `first-found`.
- **`optional: true` on the metadata interface in the netplan.** wait-online then does not wait, but ACD still runs: cloud-init's first MMDS fetch waited instead, and after the networkd restart the interface had no address.
- **A systemd-networkd drop-in with `DuplicateAddressDetection=none`.** This needs a change in each guest image, or a cloud-init step that runs after both ACD rounds.

## Consequences

- With the flag on, the guest kernel must have `CONFIG_IP_PNP`. Without it, the metadata interface gets no address and the guest cannot read its metadata.
- Until the default changes, the boot delay stays for operators who do not set the flag.
- The kernel gives the address `scope global`, not `scope link`. Tools that select addresses by scope, not by value, can now see `169.254.0.1`.
- A guest image that enables `systemd-network-generator` turns `ip=` into a networkd configuration, and ACD occurs again.
