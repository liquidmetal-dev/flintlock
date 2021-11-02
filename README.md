
# Flintlock - Create and manage the lifecycle of MicroVMs, backed by containerd.

[![GitHub](https://img.shields.io/github/license/weaveworks/flintlock)](https://img.shields.io/github/license/weaveworks/flintlock)
[![codecov](https://codecov.io/gh/weaveworks/flintlock/branch/main/graph/badge.svg?token=ZNPNRDI8Z0)](https://codecov.io/gh/weaveworks/flintlock)
[![Go Report Card](https://goreportcard.com/badge/github.com/weaveworks/flintlock)](https://goreportcard.com/report/github.com/weaveworks/flintlock)

## What is flintlock?

Flintlock is a service for creating and managing the lifecycle of microVMs on a host machine. Initially we will be supporting [Firecracker](https://firecracker-microvm.github.io/).

The primary use case for flintlock is to create microVMs on a bare-metal host where the microVMs will be used as nodes in a virtualized Kubernetes cluster. It is an essential part of [Liquid Metal](https://www.weave.works/blog/multi-cluster-kubernetes-on-microvms-for-bare-metal) and will ultimately be driven by Cluster API Provider Microvm (coming soon).

## Features

Using API requests (via gRPC or HTTP):

- Create, update, delete microVMs using Firecracker
- Manage the lifecycle of microVMs (i.e. start, stop, pause)
- Make metadata available to microVMs to support cloud-init, ignition etc
- Use OCI images for the volumes, kernel & initrd
- (coming soon) Use CNI to configure the network for the microVMs

## Documentation

See our [getting started with flintlock][quickstart] guide.

## Contributing

Contributions are welcome. Please read the [CONTRIBUTING.md][contrib] and our [Code Of Conduct][coc]. We have the [#liquid-metal](https://weave-community.slack.com/archives/C02KARWGR7S) slack channel for discussions when contributors and users.

Other interesting resources include:

- [The issue tracker][issues]
- [The list of milestones][milestones]
- [Architectural Decision Records (ADR)][adr]
- [Getting started with flintlock][quickstart]

## Getting Help

If you have any questions about, feedback for or problems with flintlock:

- [File an issue][issues].

Your feedback is always welcome!

## License

[MPL-2.0 License][license]

[quickstart]: ./docs/quick-start.md
[contrib]: ./CONTRIBUTING.md
[coc]: ./CODE_OF_CONDUCT.md
[issues]: https://github.com/weaveworks/flintlock/issues
[milestones]: https://github.com/weaveworks/flintlock/milestones
[adr]: ./docs/adr
[license]: ./LICENSE
