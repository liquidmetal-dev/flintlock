---
title: Flintlock
layout: hextra-home
---

<div class="hx:mb-4">
{{< hextra/hero-badge link="https://github.com/liquidmetal-dev/flintlock/releases/latest" >}}
  Lock, Stock, and Two Smoking MicroVMs
{{< /hextra/hero-badge >}}
</div>

<div class="hx:mt-6 hx:mb-6">
{{< hextra/hero-headline >}}
  Flintlock
{{< /hextra/hero-headline >}}
</div>

<div class="hx:mb-12">
{{< hextra/hero-subtitle >}}
  Run hardware-isolated microVMs on your own hosts through one gRPC API.&nbsp;<br class="hx:sm:block hx:hidden" />Firecracker or Cloud Hypervisor, booted from OCI images.
{{< /hextra/hero-subtitle >}}
</div>

<div class="hx:mb-12 hx:flex hx:flex-wrap hx:gap-4">
{{< hextra/hero-button text="Get started" link="/docs/getting-started/" >}}
{{< hextra/hero-button text="Read the docs" link="/docs/intro/" style="background: transparent; color: inherit; border: 1px solid currentColor;" >}}
</div>

{{< hextra/feature-grid cols="3" >}}
  {{< hextra/feature-card
    title="A kernel per workload"
    subtitle="Each microVM gets its own kernel behind KVM, so a workload can't reach its neighbours through a shared kernel the way a container can. MicroVMs still boot in seconds."
    icon="shield-check"
    link="/docs/intro/"
  >}}
  {{< hextra/feature-card
    title="Pick your hypervisor"
    subtitle="Use Firecracker or Cloud Hypervisor. Enable both on one host and choose per microVM."
    icon="switch-horizontal"
    link="/docs/getting-started/"
  >}}
  {{< hextra/feature-card
    title="Images, not disk files"
    subtitle="Kernel, initrd and volumes come from OCI images in any registry, pulled through containerd. Build them with the tools you already use."
    icon="archive"
    link="/docs/guides/images/"
  >}}
  {{< hextra/feature-card
    title="API first"
    subtitle="Create, get, list and delete microVMs over gRPC or the HTTP gateway, or use the Go client."
    icon="code"
    link="https://buf.build/liquidmetal-dev/flintlock"
  >}}
  {{< hextra/feature-card
    title="Reach into the guest"
    subtitle="Run commands in a microVM and proxy SSH to it through a guest agent over vsock. The guest doesn't need a reachable network."
    icon="terminal"
    link="/docs/grpc/services/microvmexec/v1alpha1/proto/"
  >}}
  {{< hextra/feature-card
    title="Metrics included"
    subtitle="A metrics exporter publishes microVM metrics for Prometheus to scrape."
    icon="chart-bar"
    link="/docs/guides/metrics/"
  >}}
{{< /hextra/feature-grid >}}

<div class="hx:mt-16 hx:mb-6">
{{< hextra/hero-section >}}
  Built for
{{< /hextra/hero-section >}}
</div>

{{< hextra/feature-grid cols="3" >}}
  {{< hextra/feature-card
    title="Kubernetes on bare metal"
    subtitle="Use microVMs as the nodes of virtualized Kubernetes clusters on bare-metal hosts, with Cluster API Provider Microvm."
    icon="server"
    link="https://github.com/liquidmetal-dev/cluster-api-provider-microvm"
  >}}
  {{< hextra/feature-card
    title="Sandboxes and agents"
    subtitle="Give untrusted code or AI agents a short-lived VM of their own, then drive it with exec and SSH."
    icon="cube"
  >}}
  {{< hextra/feature-card
    title="Pipelines and isolated jobs"
    subtitle="Start a clean VM for each CI job or tenant workload and delete it when the job is done."
    icon="lightning-bolt"
  >}}
{{< /hextra/feature-grid >}}

<div class="hx:mt-16 hx:mb-6">
{{< hextra/hero-section >}}
  Features
{{< /hextra/hero-section >}}
</div>

{{< hextra/feature-grid cols="3" >}}
  {{< hextra/feature-card
    title="Guest metadata"
    subtitle="Pass cloud-init or Ignition metadata to configure the guest when it boots."
  >}}
  {{< hextra/feature-card
    title="Flexible networking"
    subtitle="Attach TAP devices, optionally on a Linux bridge, or macvtap devices (Cloud Hypervisor). Use DHCP or static IP addresses."
  >}}
  {{< hextra/feature-card
    title="Shared directories"
    subtitle="Share host directories with a microVM using virtiofs (Cloud Hypervisor)."
  >}}
  {{< hextra/feature-card
    title="CPU features"
    subtitle="Choose which CPU features the guest sees."
  >}}
  {{< hextra/feature-card
    title="Guest agent"
    subtitle="Talk to an agent in the microVM over a vsock device. No guest networking needed."
  >}}
  {{< hextra/feature-card
    title="Namespaces and labels"
    subtitle="Group microVMs into namespaces, list them by namespace or name, and tag them with labels."
  >}}
{{< /hextra/feature-grid >}}

<div class="hx:mt-16 hx:mb-6">
{{< hextra/hero-section >}}
  Try it
{{< /hextra/hero-section >}}
</div>

<div class="content hx:w-full">

With `flintlockd` running, create a microVM with [fl](https://github.com/liquidmetal-dev/fl). The kernel and root volume are OCI images:

```bash
fl microvm create --host 192.168.1.66:9090 --name my-microvm \
  --kernel-image ghcr.io/liquidmetal-dev/firecracker-kernel:6.1 \
  --root-image ghcr.io/liquidmetal-dev/ubuntu:24.04 \
  --allow-guest-agent
```

Then look it up, run a command in it, or open a shell. For `exec` and `ssh`, start `flintlockd` with `--enable-exec-api` and `--enable-ssh-proxy-api`:

```bash
fl microvm get --host 192.168.1.66:9090 <vmid>
fl microvm exec --host 192.168.1.66:9090 <vmid> -- uname -a
fl microvm ssh --host 192.168.1.66:9090 <vmid>
```

The [getting started tutorial](/docs/getting-started/) walks through setting up a host from scratch.

</div>

<div class="hx:mt-16 hx:mb-6">
{{< hextra/hero-section >}}
  The Liquid Metal ecosystem
{{< /hextra/hero-section >}}
</div>

{{< hextra/feature-grid cols="2" >}}
  {{< hextra/feature-card
    title="Battery"
    subtitle="A warm pool manager that keeps pools of pre-booted microVMs ready to be claimed."
    icon="sparkles"
    link="https://github.com/liquidmetal-dev/battery"
  >}}
  {{< hextra/feature-card
    title="Brigade"
    subtitle="A distributed orchestrator that schedules microVMs across a fleet of flintlock hosts."
    icon="globe-alt"
    link="https://github.com/liquidmetal-dev/brigade"
  >}}
  {{< hextra/feature-card
    title="Cluster API Provider Microvm"
    subtitle="A Cluster API provider that creates Kubernetes clusters with microVMs as the nodes."
    icon="puzzle"
    link="https://github.com/liquidmetal-dev/cluster-api-provider-microvm"
  >}}
  {{< hextra/feature-card
    title="fl"
    subtitle="A command-line tool for managing microVMs on flintlock hosts."
    icon="terminal"
    link="https://github.com/liquidmetal-dev/fl"
  >}}
{{< /hextra/feature-grid >}}

<div class="content hx:mt-16 hx:w-full">

Questions or ideas? Find us in [#liquidmetal](https://cloud-native.slack.com/archives/C07B5R5BLBZ) on the CNCF Slack, or read the [contributing guide](https://github.com/liquidmetal-dev/flintlock/blob/main/CONTRIBUTING.md) on GitHub.

</div>
