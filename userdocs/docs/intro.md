---
title: Introduction
---

# Introduction

:::warning site under construction
:::

## What is flintlock?

Flintlock is a service for creating and managing the lifecycle of microVMs on a
host machine. Initially we will be supporting [Firecracker][firecracker],
with an aim to default to [Cloud Hypervisor][ch] in the future.

The primary use case for flintlock is to create microVMs on a bare-metal host
where the microVMs will be used as nodes in a virtualized Kubernetes cluster.
It is an essential part of [Liquid Metal][liquid-metal] and will ultimately be
driven by [Cluster API Provider Microvm][capmvm].

## Features

Using API requests (via [gRPC][proto] or <a href="/flintlock-api" target="_blank">HTTP</a>):

- Create, update, delete microVMs using Firecracker
- Manage the lifecycle of microVMs (i.e. start, stop, pause)
- Configure microVM metadata via cloud-init, ignition etc
- Use OCI images for microVM volumes, kernel and initrd
- (coming soon) Use CNI to configure the network for the microVMs

## Liquid Metal

To learn more about using Flintlock MicroVMs in a Kubernetes cluster, check
out the [official Liquid Metal docs][lm].


[ch]: https://www.cloudhypervisor.org/
[capmvm]: https://github.com/weaveworks-liquidmetal/cluster-api-provider-microvm
[proto]: https://buf.build/weaveworks-liquidmetal/flintlock
[lm]: https://weaveworks-liquidmetal.github.io/site/
[firecracker]: https://firecracker-microvm.github.io/
[liquid-metal]: https://www.weave.works/blog/multi-cluster-kubernetes-on-microvms-for-bare-metal
