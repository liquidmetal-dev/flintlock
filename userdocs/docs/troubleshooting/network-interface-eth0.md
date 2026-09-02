---
title: Flintlockd
---

## Error: misleading `network_interfaces` error when creating a MicroVM

Example error:

```
ERRO[0003] failed to reconcile microvm ns1/mvm0:
  creating microvm:
    creating firecracker config:
      updating firecracker network-interfaces config:
        firecracker api rejected config: network_interfaces: invalid or missing
```

The exact wording depends on the Firecracker version, but the error points
at network interfaces in a way that looks like a flintlock configuration
mistake with the `network-interfaces`/`network_interfaces` field names. It
isn't: those names are correct and unrelated to the real problem.

The actual cause is that one of the MicroVM's `NetworkInterface` entries has
its `device_id` set to `eth0`. Firecracker reserves `eth0` for its own use,
so a guest interface named `eth0` clashes with it and the create fails with
this misleading error.

This only happens when `guest_mac` is **not** set on that interface: without
a MAC address to match on, flintlock falls back to matching the guest
interface by name, using `device_id` as that name. If `guest_mac` is set,
`device_id` is just an opaque identifier and this problem does not occur.

### Fix

Either:

- Use a different `device_id` for the interface, e.g. `eth1`, or
- Set `guest_mac` on the interface so it is matched by MAC address instead
  of by name.
