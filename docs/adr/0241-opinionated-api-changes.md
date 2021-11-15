# Opinionated API Changes

* Status: accepted
* Date: 2021-11-10
* Authors: @jmickey
* Deciders: @jmickey @richardcase @Callisto13 @yitsushi
* ADR Discussion: https://github.com/weaveworks/flintlock/discussions/241

## Context

Flintlock requires users to specify all aspects of the MicroVM spec. If something (e.g. metadata network interface) is not included in the MicroVM spec, then it won't be created. This proposal details some possible opinionated defaults for MicroVM specifications in cases where an explicit configuration is not provided by the user.

### Scope

The scope of this proposal is not to decide on what aspects of MicroVM specifications are immutable / non-configurable. Rather it is to explore which potential aspects of the MicroVM specification can be provided by default, but also overridden/customised by the user if they so wish to.

### Default Configurations

Below are some proposed default configurations, along with default values where relevant.

| Name | Description | Model | Value |
|----------|---|---------|---------|
| MicroVM Resources <img width="150px"/> | Provide default values for MicroVM VCPUs and RAM. This can be difficult to get right, but is even supported in Firecracker itself. However, Firecracker defaults to 1 VCPU and 128MB of RAM, which is probably a bit small for our particular use case. | `MicroVM.VCPU`<br />`MicroVM.MemoryInMb` <img width="400px"/> | `vcpu: 2`<br />`memory_in_mb: 1024` <img width="400px"/> |
| Default Namespace | If a namespace is not provided than default to using the `default` namespace | `MicroVM.Namespace` | `default` |
| Metadata network interface | Automatically generate metadata network interface with address of `169.254.0.1/16` and `allow_metadata_req: true` | `MicroVM.NetworkInterfaces` | `169.254.0.1/16` |
| Remove allow metadata requests  | If the metadata network interface is created automatically then we should remove the `allow_metadata_req` option on the network interface model | `MicroVM.NetworkInterfaces` |  |
| Network interface name | Automatically generate the network interface name when one is not provided. | `MicroVM.NetworkInterfaces[*].GuestDeviceName` | |
| `RootVolume` and `AdditionalVolumes[]` | Separate the `Volumes` field in the model into two fields. `RootVolume` specifically for the root volume, allowing us to hide the `is_root` flag, and `AdditionalVolumes[]` for non-root volumes. | `MicroVM.Volumes` | `MicroVM.RootVolume` and `MicroVM.AdditionalVolumes[]` |

### Unknowns/Discussion

- ~~Can we generate volume names?~~
- ~~If a volume is marked as `is_root: true`, can we automatically mount it at `/` if `mount_point` is not specified?~~

### Request Example

With the defaults as described in Default Configurations, a minimal MicroVM create request would look as follows:

```json
{
  "microvm": {
    "id": "mvm1",
    "kernel": {
      "image": "docker.io/richardcase/ubuntu-bionic-kernel:0.0.11",
      "filename": "vmlinux"
    },
    "initrd": {
      "image": "docker.io/richardcase/ubuntu-bionic-kernel:0.0.11",
      "filename": "initrd-generic"
    },
    "interfaces": [
      {
        "type": 0
      }
    ],
  }
}
```

## Decision

Implement the following changes to the Flintlock API:

| Name | Description | Model | Value |
|----------|---|---------|---------|
| MicroVM Resources <img width="100px"/> | Provide default values for MicroVM VCPUs and RAM. This can be difficult to get right, but is even supported in Firecracker itself. However, Firecracker defaults to 1 VCPU and 128MB of RAM, which is probably a bit small for our particular use case. | `MicroVM.VCPU`<br />`MicroVM.MemoryInMb` <img width="400px"/> | `vcpu: 2`<br />`memory_in_mb: 1024` <img width="400px"/> |
| Default Namespace | If a namespace is not provided than default to using the `default` namespace | `MicroVM.Namespace` | `default` |
| Metadata network interface | Automatically generate metadata network interface with address of `169.254.0.1/16` and `allow_metadata_req: true` | `MicroVM.NetworkInterfaces` | `169.254.0.1/16` |
| Remove allow metadata requests  | If the metadata network interface is created automatically then we should remove the `allow_metadata_req` option on the network interface model | `MicroVM.NetworkInterfaces` |  |
| Network interface name | Automatically generate the network interface name when one is not provided. | `MicroVM.NetworkInterfaces[*].GuestDeviceName` | |
| `RootVolume` and `AdditionalVolumes[]` | Separate the `Volumes` field in the model into two fields. `RootVolume` specifically for the root volume, allowing us to hide the `is_root` flag, and `AdditionalVolumes[]` for non-root volumes. | `MicroVM.Volumes` | `MicroVM.RootVolume` and `MicroVM.AdditionalVolumes[]` |

Note: This proposal _specifically_ avoids diving too deep into implementation details. E.g. Automatically generating a network device name will require us to validate that there are no conflicts in the name we use. These implementation details belong within the scope of the implementation phase and PR discussion.

## Consequences
<!-- Whats the result or impact of this decision. Does anything need to change and are new GitHub issues created as a result -->