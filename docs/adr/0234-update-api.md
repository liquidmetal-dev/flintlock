# Firecracker Patch Limitations & The Flintlock Update API

- Status: accepted
- Date: 2021-11-08
- Authors: @jmickey
- Deciders: @jmickey @richardcase @Callisto13 @yitsushi
- ADR Discussion: https://github.com/liquidmetal-dev/flintlock/discussions/234

## Context

Users of Flintlock may want to make changes to MicroVMs that are already provisioned and running. In order to support MicroVM updates and provide users with an API to achieve this we need to understand what the limitations are within the MicroVM providers. Flintlocks initial MicroVM provider is [Firecracker](https://github.com/firecracker-microvm/firecracker).

Firecracker provides [Swagger API definitions](https://github.com/firecracker-microvm/firecracker/blob/main/src/api_server/swagger/firecracker.yaml) that outlines the different operations that are possible, and when those operations are valid.

Unfortunately, very few operations are valid **after** a MicroVM has been started. These include:

- [Network interface rate limiting](https://github.com/firecracker-microvm/firecracker/blob/main/docs/api_requests/patch-network-interface.md).
- [Volume rate limiting, and "hot-swapping" the underlying raw block device on the host](https://github.com/firecracker-microvm/firecracker/blob/main/docs/api_requests/patch-block.md). However, there are limitations with this:
  - The guest must not have mounted the device.
  - The guest must not read or write from the raw block device during the update sequence.
- [MicroVM metadata service data store.](https://github.com/firecracker-microvm/firecracker/blob/main/src/api_server/swagger/firecracker.yaml#L394)
- [Balloon devices.](https://github.com/firecracker-microvm/firecracker/blob/main/src/api_server/swagger/firecracker.yaml#L105)

As a result of discovering these limitations, the `flintlock` update gRPC API and update implementation code was removed in #222 and the need to revisit how we would handle updates moving forward was identified.

Moving forward there are a few possible directions that could be taken:

1. **Provide no support for in-place updates**. The primary consumer of the `flintlock` API will be the MicroVM Cluster API (CAPI) provider, which doesn't support updates. As a result it would be perfectly valid for us to decide that we won't support any in-place update capabilities, and instead all changes to MicroVM specs will result in recreation.
2. **Implement in-place updates for valid operations**. Rate limiting, limited unmounted volume changes, metadata, and balloon devices are able to updated in-place.
3. **Only update metadata**. The metadata service data store is probably the most logical place where in-place updates would make sense within the context of possible consumers (CAPI).

## Decision

**Provide no support for in-place updates**.

The primary consumer of the `flintlock` API will be the MicroVM Cluster API (CAPI) provider, which doesn't support updates. As a result it would be perfectly valid for us to decide that we won't support any in-place update capabilities, and instead all changes to MicroVM specs will result in recreation.

## Consequences

Having to recreate MicroVMs in order to support spec updates could possibly lead to issues where the host does not have sufficient resources in order to support `CreateBeforeDelete` style updates, where a new MicroVM is created, workloads are migrated, and the old MicroVM is removed. However, this is less of a consequence of this decision, and more a consequence of the inherent limitations of the Firecracker MicroVM provider.
