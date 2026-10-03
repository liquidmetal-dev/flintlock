---
title: Introduction
---

# Introduction

:::warning site under construction
:::

## What is flintlock?

Flintlock is a service for creating and managing the lifecycle of microVMs on a
host machine. We support the [Cloud Hypervisor][ch] and
[Firecracker][firecracker] VMMs.

The primary use case for flintlock is to create microVMs on a bare-metal host
where the microVMs will be used as nodes in a virtualized Kubernetes cluster.
It is an essential part of [Liquid Metal][liquid-metal] and can be orchestrated
by:

- [Battery][battery]: a warm pool manager that keeps pools of pre-booted
  microVMs ready to be claimed.
- [Brigade][brigade]: a distributed orchestrator that schedules microVMs across
  a fleet of flintlock hosts.
- [Cluster API Provider Microvm][capmvm]: a Cluster API provider that creates
  Kubernetes clusters with microVMs as the nodes.

## Features

Using API requests (via [gRPC][proto] or <a href="/flintlock-api" target="_blank">HTTP</a>):

- Create, get, list and delete microVMs
- Run microVMs with Firecracker or Cloud Hypervisor, chosen per microVM
- Configure microVM metadata via cloud-init, ignition etc
- Use OCI images for microVM volumes, kernel and initrd
- Attach network interfaces using TAP devices (optionally on a Linux bridge) or
  macvtap devices (Cloud Hypervisor only), with DHCP or static IP addresses
- Share host directories with a microVM using virtiofs (Cloud Hypervisor only)
- Customise the CPU features presented to the guest
- Talk to a guest agent in the microVM over a vsock device
- Expose microVM metrics for collection by Prometheus

## Liquid Metal

To learn more about using Flintlock MicroVMs in a Kubernetes cluster, check
out the [official Liquid Metal docs][lm].

[ch]: https://www.cloudhypervisor.org/
[battery]: https://github.com/liquidmetal-dev/battery
[brigade]: https://github.com/liquidmetal-dev/brigade
[capmvm]: https://github.com/liquidmetal-dev/cluster-api-provider-microvm
[proto]: https://buf.build/liquidmetal-dev/flintlock
[lm]: https://www.liquidmetal.dev
[firecracker]: https://firecracker-microvm.github.io/
[liquid-metal]: https://www.weave.works/blog/multi-cluster-kubernetes-on-microvms-for-bare-metal
