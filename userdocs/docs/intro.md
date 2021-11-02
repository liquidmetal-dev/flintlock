---
sidebar_position: 1
---

# Introduction

## What is flintlock?

Flintlock is a service for creating and managing the lifecycle of microVMs on a
host machine. Initially we will be supporting [Firecracker][firecracker]. 

The primary use case for flintlock is to create microVMs on a bare-metal host
where the microVMs will be used as nodes in a virtualized Kubernetes cluster.
It is an essential part of [Liquid Metal][liquid-metal] and will ultimately be
driven by Cluster API Provider Microvm (coming soon).

[firecracker]: https://firecracker-microvm.github.io/
[liquid-metal]: https://www.weave.works/blog/multi-cluster-kubernetes-on-microvms-for-bare-metal

## Features

Using API requests (via gRPC or HTTP):
- Create, update, delete microVMs using Firecracker
- Manage the lifecycle of microVMs (i.e. start, stop, pause)
- Make metadata available to microVMs to support cloud-init, ignition etc
- Use OCI images for the volumes, kernel & initrd
- (coming soon) Use CNI to configure the network for the microVMs

## Documentation

:::info
Flintlock and flintlock tests are only compatible with Linux. We recommend that
non-linux users provision a [Linux VM][vagrant] in which to work.
:::

See our [getting started with flintlock][getting-started] guide.

[vagrant]: ./getting-started/extras/use-vagrant
[getting-started]: ./getting-started/basics/configuring-network
