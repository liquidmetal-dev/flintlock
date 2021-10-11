
# 🆁🅴🅸🅶🅽🅸🆃🅴 - the microvm service

[![GitHub](https://img.shields.io/github/license/weaveworks/reignite)](https://img.shields.io/github/license/weaveworks/reignite)
[![codecov](https://codecov.io/gh/weaveworks/reignite/branch/main/graph/badge.svg?token=ZNPNRDI8Z0)](https://codecov.io/gh/weaveworks/reignite)
[![Go Report Card](https://goreportcard.com/badge/github.com/weaveworks/reignite)](https://goreportcard.com/report/github.com/weaveworks/reignite)

## What is regnite?

Reignite is a service for creating and managing the lifecycle of microVMs on a host machine. Initially we will be supporting [Firecracker](https://firecracker-microvm.github.io/). 

The primary use case for reignite is to create microVMs on a bare-metal host where the microVMs will be used as nodes in a virtualized Kubernetes cluster. It is an essential part of [Liquid Metal](https://www.weave.works/blog/multi-cluster-kubernetes-on-microvms-for-bare-metal) and will ultimately be driven by Cluster API Provider Microvm (coming soon).

## Features

Using API requests (via gRPC or HTTP):
- Create, update, delete microVMs using Firecracker
- Manage the lifecycle of microVMs (i.e. start, stop, pause)
- Make metadata available to microVMs to support cloud-init, ignition etc
- Use OCI images for the volumes, kernel & initrd
- (coming soon) Use CNI to configure the network for the microVMs

## Documentation

See our [getting started with reignite][quickstart] guide.

## Contributing

Contributions are welcome. Please read the [CONTRIBUTING.md][contrib] and our [Code Of Conduct][coc].

Other interesting resources include:

* [The issue tracker][issues]
* [The list of milestones][milestones]
* [Architectural Decision Records (ADR)][adr]
* [Getting started with reignite][quickstart]

## Getting Help

If you have any questions about, feedback for or problems with reignite:

* [File an issue][issues].

Your feedback is always welcome!

## License

[MPL-2.0 License][license]


[quickstart]: ./docs/quick-start.md
[contrib]: ./CONTRIBUTING.md
[coc]: ./CODE_OF_CONDUCT.md
[issues]: https://github.com/weaveworks/reignite/issues
[milestones]: https://github.com/weaveworks/reignite/milestones
[adr]: ./docs/adr
[license]: ./LICENSE
