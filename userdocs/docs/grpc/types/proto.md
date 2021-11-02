# Protocol Documentation
<a name="top"></a>

## Table of Contents

- [types/microvm.proto](#types/microvm.proto)
    - [ContainerVolumeSource](#flintlock.types.ContainerVolumeSource)
    - [Initrd](#flintlock.types.Initrd)
    - [Kernel](#flintlock.types.Kernel)
    - [MicroVM](#flintlock.types.MicroVM)
    - [MicroVMSpec](#flintlock.types.MicroVMSpec)
    - [MicroVMSpec.LabelsEntry](#flintlock.types.MicroVMSpec.LabelsEntry)
    - [MicroVMSpec.MetadataEntry](#flintlock.types.MicroVMSpec.MetadataEntry)
    - [MicroVMStatus](#flintlock.types.MicroVMStatus)
    - [MicroVMStatus.NetworkInterfacesEntry](#flintlock.types.MicroVMStatus.NetworkInterfacesEntry)
    - [MicroVMStatus.VolumesEntry](#flintlock.types.MicroVMStatus.VolumesEntry)
    - [Mount](#flintlock.types.Mount)
    - [NetworkInterface](#flintlock.types.NetworkInterface)
    - [NetworkInterfaceStatus](#flintlock.types.NetworkInterfaceStatus)
    - [Volume](#flintlock.types.Volume)
    - [VolumeSource](#flintlock.types.VolumeSource)
    - [VolumeStatus](#flintlock.types.VolumeStatus)
  
    - [MicroVMStatus.MicroVMState](#flintlock.types.MicroVMStatus.MicroVMState)
    - [Mount.MountType](#flintlock.types.Mount.MountType)
    - [NetworkInterface.IfaceType](#flintlock.types.NetworkInterface.IfaceType)
  
- [Scalar Value Types](#scalar-value-types)



<a name="types/microvm.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## types/microvm.proto



<a name="flintlock.types.ContainerVolumeSource"></a>

### ContainerVolumeSource
ContainerVolumeSource represents the details of a volume coming from a OCI image.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| image | [string](#string) |  | Image specifies teh conatiner image to use for the volume. |






<a name="flintlock.types.Initrd"></a>

### Initrd
Initrd represents the configuration for the initial ramdisk.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| image | [string](#string) |  | Image is the container image to use. |
| filename | [string](#string) | optional | Filename is used to specify the name of the kernel file in the Image. Defaults to initrd |






<a name="flintlock.types.Kernel"></a>

### Kernel
Kernel represents the configuration for a kernel.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| image | [string](#string) |  | Image is the container image to use. |
| cmdline | [string](#string) |  | Cmdline is the kernel command line args. |
| filename | [string](#string) | optional | Filename is used to specify the name of the kernel file in the Image. |
| add_network_config | [bool](#bool) |  | AddNetworkConfig if set to true indicates that the network-config kernel argument should be generated. |






<a name="flintlock.types.MicroVM"></a>

### MicroVM
MicroVM represents a microvm machine that is created via a provider.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| version | [int32](#int32) |  |  |
| spec | [MicroVMSpec](#flintlock.types.MicroVMSpec) |  | Spec is the specification of the microvm. |
| status | [MicroVMStatus](#flintlock.types.MicroVMStatus) |  | Status is the runtime status of the microvm. |






<a name="flintlock.types.MicroVMSpec"></a>

### MicroVMSpec
MicroVMSpec represents the specification for a microvm.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | ID is the identifier of the microvm. If this empty at creation time a ID will be automatically generated. |
| namespace | [string](#string) |  | Namespace is the name of the namespace the microvm belongs to. |
| labels | [MicroVMSpec.LabelsEntry](#flintlock.types.MicroVMSpec.LabelsEntry) | repeated | Labels allows you to include extra data for the microvms. |
| vcpu | [int32](#int32) |  | VCPU specifies how many vcpu the machine will be allocated. |
| memory_in_mb | [int32](#int32) |  | MemoryInMb is the amount of memory in megabytes that the machine will be allocated. |
| kernel | [Kernel](#flintlock.types.Kernel) |  | Kernel is the details of the kernel to use . |
| initrd | [Initrd](#flintlock.types.Initrd) | optional | Initrd is the optional details of the initial ramdisk. |
| volumes | [Volume](#flintlock.types.Volume) | repeated | Volumes specifies the volumes to be attached to the microvm. There must be at lease one volume that will act as the root volume. |
| interfaces | [NetworkInterface](#flintlock.types.NetworkInterface) | repeated | Interfaces specifies the network interfaces to be attached to the microvm. |
| metadata | [MicroVMSpec.MetadataEntry](#flintlock.types.MicroVMSpec.MetadataEntry) | repeated | Metadata allows you to specify data to be added to the metadata service. The key is the name of the metadata item and the value is the base64 encoded contents of the metadata. |
| created_at | [google.protobuf.Timestamp](#google.protobuf.Timestamp) |  | CreatedAt indicates the time the microvm was created at. |
| updated_at | [google.protobuf.Timestamp](#google.protobuf.Timestamp) |  | UpdatedAt indicates the time the microvm was last updated. |
| deleted_at | [google.protobuf.Timestamp](#google.protobuf.Timestamp) |  | DeletedAt indicates the time the microvm was marked as deleted. |






<a name="flintlock.types.MicroVMSpec.LabelsEntry"></a>

### MicroVMSpec.LabelsEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="flintlock.types.MicroVMSpec.MetadataEntry"></a>

### MicroVMSpec.MetadataEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="flintlock.types.MicroVMStatus"></a>

### MicroVMStatus
MicroVMStatus contains the runtime status of the microvm.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| state | [MicroVMStatus.MicroVMState](#flintlock.types.MicroVMStatus.MicroVMState) |  | State stores information about the last known state of the vm and the spec. |
| volumes | [MicroVMStatus.VolumesEntry](#flintlock.types.MicroVMStatus.VolumesEntry) | repeated | Volumes holds the status of the volumes. |
| kernel_mount | [Mount](#flintlock.types.Mount) |  | KernelMount holds the status of the kernel mount point. |
| initrd_mount | [Mount](#flintlock.types.Mount) |  | InitrdMount holds the status of the initrd mount point. |
| network_interfaces | [MicroVMStatus.NetworkInterfacesEntry](#flintlock.types.MicroVMStatus.NetworkInterfacesEntry) | repeated | NetworkInterfaces holds the status of the network interfaces. |
| retry | [int32](#int32) |  | Retry is a counter about how many times we retried to reconcile. |






<a name="flintlock.types.MicroVMStatus.NetworkInterfacesEntry"></a>

### MicroVMStatus.NetworkInterfacesEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [NetworkInterfaceStatus](#flintlock.types.NetworkInterfaceStatus) |  |  |






<a name="flintlock.types.MicroVMStatus.VolumesEntry"></a>

### MicroVMStatus.VolumesEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [VolumeStatus](#flintlock.types.VolumeStatus) |  |  |






<a name="flintlock.types.Mount"></a>

### Mount
Mount represents a volume mount point.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| type | [Mount.MountType](#flintlock.types.Mount.MountType) |  | Type specifies the type of the mount (e.g. device or directory). |
| source | [string](#string) |  | Source is the location of the mounted volume. |






<a name="flintlock.types.NetworkInterface"></a>

### NetworkInterface



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| guest_device_name | [string](#string) |  | GuestDeviceName is the name of the network interface to create in the microvm. |
| type | [NetworkInterface.IfaceType](#flintlock.types.NetworkInterface.IfaceType) |  | IfaceType specifies the type of network interface to create for use by the guest. |
| allow_metadata_req | [bool](#bool) |  | AllowMetadataReq indicates if the network interface allows metadata requests. |
| guest_mac | [string](#string) | optional | GuestMAC allows the specifying of a specifi MAC address to use for the interface. If not supplied a autogenerated MAC address will be used. |
| address | [string](#string) | optional | Address is an optional IP address to assign to this interface. If not supplied then DHCP will be used. |






<a name="flintlock.types.NetworkInterfaceStatus"></a>

### NetworkInterfaceStatus



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| host_device_name | [string](#string) |  | HostDeviceName is the name of the network interface used from the host. This will be a tuntap or macvtap interface. |
| index | [int32](#int32) |  | Index is the index of the network interface on the host. |
| mac_address | [string](#string) |  | MACAddress is the MAC address of the host interface. |






<a name="flintlock.types.Volume"></a>

### Volume
Volume represents the configuration for a volume to be attached to a microvm.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  | ID is the uinique identifier of the volume. |
| is_root | [bool](#bool) |  | IsRoot specifies that the volume is to be used as the root volume. A machine must have a root volume. |
| is_read_only | [bool](#bool) |  | IsReadOnly specifies that the volume is to be mounted readonly. |
| mount_point | [string](#string) |  | MountPoint is the mount point for the volume in the microvm. |
| source | [VolumeSource](#flintlock.types.VolumeSource) |  | Source is where the volume will be sourced from. |
| partition_id | [string](#string) | optional | PartitionID is the uuid of the boot partition. |
| size_in_mb | [int32](#int32) | optional | Size is the size to resize this volume to.

TODO: add rate limiting |






<a name="flintlock.types.VolumeSource"></a>

### VolumeSource
VolumeSource is the source of a volume. Based loosely on the volumes in Kubernetes Pod specs.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| container_source | [string](#string) | optional | Container is used to specify a source of a volume as a OCI container.

TODO: add CSI |






<a name="flintlock.types.VolumeStatus"></a>

### VolumeStatus



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| mount | [Mount](#flintlock.types.Mount) |  | Mount represents a volume mount point. |





 


<a name="flintlock.types.MicroVMStatus.MicroVMState"></a>

### MicroVMStatus.MicroVMState


| Name | Number | Description |
| ---- | ------ | ----------- |
| PENDING | 0 |  |
| CREATED | 1 |  |
| FAILED | 2 |  |



<a name="flintlock.types.Mount.MountType"></a>

### Mount.MountType


| Name | Number | Description |
| ---- | ------ | ----------- |
| DEV | 0 |  |
| HOSTPATH | 1 |  |



<a name="flintlock.types.NetworkInterface.IfaceType"></a>

### NetworkInterface.IfaceType


| Name | Number | Description |
| ---- | ------ | ----------- |
| MACVTAP | 0 | MACVTAP represents a network interface that is macvtap. |
| TAP | 1 | TAP represents a network interface that is a tap. |


 

 

 



## Scalar Value Types

| .proto Type | Notes | C++ | Java | Python | Go | C# | PHP | Ruby |
| ----------- | ----- | --- | ---- | ------ | -- | -- | --- | ---- |
| <a name="double" /> double |  | double | double | float | float64 | double | float | Float |
| <a name="float" /> float |  | float | float | float | float32 | float | float | Float |
| <a name="int32" /> int32 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint32 instead. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="int64" /> int64 | Uses variable-length encoding. Inefficient for encoding negative numbers – if your field is likely to have negative values, use sint64 instead. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="uint32" /> uint32 | Uses variable-length encoding. | uint32 | int | int/long | uint32 | uint | integer | Bignum or Fixnum (as required) |
| <a name="uint64" /> uint64 | Uses variable-length encoding. | uint64 | long | int/long | uint64 | ulong | integer/string | Bignum or Fixnum (as required) |
| <a name="sint32" /> sint32 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int32s. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="sint64" /> sint64 | Uses variable-length encoding. Signed int value. These more efficiently encode negative numbers than regular int64s. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="fixed32" /> fixed32 | Always four bytes. More efficient than uint32 if values are often greater than 2^28. | uint32 | int | int | uint32 | uint | integer | Bignum or Fixnum (as required) |
| <a name="fixed64" /> fixed64 | Always eight bytes. More efficient than uint64 if values are often greater than 2^56. | uint64 | long | int/long | uint64 | ulong | integer/string | Bignum |
| <a name="sfixed32" /> sfixed32 | Always four bytes. | int32 | int | int | int32 | int | integer | Bignum or Fixnum (as required) |
| <a name="sfixed64" /> sfixed64 | Always eight bytes. | int64 | long | int/long | int64 | long | integer/string | Bignum |
| <a name="bool" /> bool |  | bool | boolean | boolean | bool | bool | boolean | TrueClass/FalseClass |
| <a name="string" /> string | A string must always contain UTF-8 encoded or 7-bit ASCII text. | string | String | str/unicode | string | string | string | String (UTF-8) |
| <a name="bytes" /> bytes | May contain any arbitrary sequence of bytes. | string | ByteString | str | []byte | ByteString | string | String (ASCII-8BIT) |

