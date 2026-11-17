---
weight: 8
title: Guest memory
---

A guest keeps every page of memory it has touched until its microVM is
deleted, the page cache included. So a busy microVM soon holds its full
`memory_in_mb` on the host, even when the guest has memory free. This page
shows how a microVM can give that memory back with a balloon and free page
reporting.

## Enable the balloon

Set `balloon` in the microVM spec:

```yaml
vcpu: 2
memory_in_mb: 4096
balloon:
  free_page_reporting: true
```

With `balloon` set, flintlock gives the VMM a balloon device that starts
empty and deflates when the guest runs out of memory. With
`free_page_reporting`, the guest reports the pages it frees, and the VMM
returns them to the host. A balloon without `free_page_reporting` takes no
memory and returns none.

Without `balloon`, the microVM has no balloon device, as before.

## What each provider gets

| Provider | Configuration |
| -------- | ------------- |
| Firecracker | A `balloon` in the config file: `amount_mib: 0`, `deflate_on_oom: true`, and `free_page_reporting` from the spec. |
| Cloud Hypervisor | `--balloon size=0,deflate_on_oom=on`, and `free_page_reporting=on` when the spec asks for it. |

flintlock's e2e tests run the balloon on Firecracker v1.16 and Cloud
Hypervisor v54.

## Requirements

- The guest kernel needs `CONFIG_VIRTIO_BALLOON` and `CONFIG_PAGE_REPORTING`.
  The kernels that flintlock's e2e tests boot,
  `ghcr.io/liquidmetal-dev/firecracker-kernel:6.1` and
  `ghcr.io/liquidmetal-dev/cloudhypervisor-kernel-bin:5.12`, have both.
- A flintlockd from before the balloon accepts a spec with `balloon` and
  drops the field. The microVM then boots with no balloon. The spec that
  `CreateMicroVM` and `GetMicroVM` return shows what flintlockd stored, so
  a client can check that it still has `balloon`.

## Check that it works

In the guest, the balloon driver binds a virtio device. Bit 5 of its
features, `VIRTIO_BALLOON_F_REPORTING`, is `1` when free page reporting is
on:

```bash
d=$(ls /sys/bus/virtio/drivers/virtio_balloon | grep '^virtio')
cut -c6 /sys/bus/virtio/devices/$d/features
```

On the host, the VMM's resident set drops a few seconds after the guest
frees memory. Compare `VmRSS` in `/proc/<pid>/status` of the VMM process
before and after the guest frees a large block, for example a file on a
tmpfs. Not all of the freed memory comes back: see the minimum order below.

## Performance

Free page reporting trades some guest CPU and memory latency for host
memory. The cost depends on the workload.

- The guest reports free ranges about 2 seconds after it frees them. The VMM
  drops each range with `madvise(MADV_DONTNEED)`.
- When the guest uses a reported page again, the host has to supply a new
  zeroed page. A workload that frees and reuses memory in quick cycles pays
  this each time, and the reports use some CPU in the guest and the VMM.
- The guest reports only ranges of at least a minimum page order. The guest
  kernel's `page_reporting_order` module parameter sets it. A
  higher order gives fewer, larger reports and returns less memory. A lower
  order returns more memory for more reports.

## Limits

- Free page reporting is set before boot. It cannot be turned on or off
  while the microVM runs. To change it, create the microVM again.
- The guest returns only memory it frees. The page cache stays in use until
  the guest drops it, for example with
  `sync; echo 3 > /proc/sys/vm/drop_caches`.
- With memory backed by huge pages, 4 KiB reports cannot free a huge page,
  so the host's memory use does not drop. flintlock does not use huge pages
  today.
- The balloon is best effort, and the guest can ignore it. A host must still
  cope when every guest grows to its full `memory_in_mb` at once. See
  [Size the host's memory](/docs/guides/production/#size-the-hosts-memory).

## More

- Firecracker: [ballooning](https://github.com/firecracker-microvm/firecracker/blob/v1.16.1/docs/ballooning.md)
- Cloud Hypervisor: [balloon](https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/docs/balloon.md)
