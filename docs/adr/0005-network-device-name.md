# 5. Network Device Name on Host

* Status: pending   // will be updated after PR review
* Date: 2021-11-08
* Authors: @yitsushi
* Deciders: @Callisto13 @jmickey @richardcase @yitsushi

## Context

With the POC, network interface names are generated from the name of the
MicroVM and its Namespace. It works for short VM name and Namespace
combinations, but device names can't be longer than 15 bytes[^1][^2].

Because of this limitation, we have to find a better way to generate network
device name on the host.

Considered options:

* Generate a UUID and use the first N bytes.
* Generate a hash of the Name and Namespace combination and use the first N
  bytes.
* Generate a random value.

[^1]: https://elixir.bootlin.com/linux/v5.6/source/include/linux/netdevice.h#L1826
[^2]: https://elixir.bootlin.com/linux/v5.6/source/include/uapi/linux/if.h#L33

## Decision

Following the device name generator in docker-ce[^3], we decided to use a random
value. Docker-ce tries to generate a name 3 times, if the generated name is
already taken. To reduce possible failures, we decided to retry 5 times,
it's still not slow, but gives more opportunities on machines with a lot of
network devices.

[^3]: https://github.com/docker/docker-ce/blob/1093a93b336461032352e776893afefc2cf3a50d/components/engine/libnetwork/netutils/utils.go#L126

## Consequences

It is not possible to determine which MicroVM is the owner of the network
device from its name and we have to query external resources to see if given
resource is in use or not.

Resources: MicroVM API, Flitlock API

As a result, it is possible to leak resources, when the MicroVM deletion failed
and we lost track of a network device status from MicroVMSpec. For that reason,
[Resource cleanup ADR #90][issue-90] priority might be raised to higher priority.

[issue-90]: https://github.com/weaveworks/flintlock/issues/90
