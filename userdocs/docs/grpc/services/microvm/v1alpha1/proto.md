# Protocol Documentation
<a name="top"></a>

## Table of Contents

- [services/microvm/v1alpha1/microvms.proto](#services/microvm/v1alpha1/microvms.proto)
    - [CreateMicroVMRequest](#microvm.services.api.v1alpha1.CreateMicroVMRequest)
    - [CreateMicroVMRequest.MetadataEntry](#microvm.services.api.v1alpha1.CreateMicroVMRequest.MetadataEntry)
    - [CreateMicroVMResponse](#microvm.services.api.v1alpha1.CreateMicroVMResponse)
    - [DeleteMicroVMRequest](#microvm.services.api.v1alpha1.DeleteMicroVMRequest)
    - [GetMicroVMRequest](#microvm.services.api.v1alpha1.GetMicroVMRequest)
    - [GetMicroVMResponse](#microvm.services.api.v1alpha1.GetMicroVMResponse)
    - [ListMessage](#microvm.services.api.v1alpha1.ListMessage)
    - [ListMicroVMsRequest](#microvm.services.api.v1alpha1.ListMicroVMsRequest)
    - [ListMicroVMsResponse](#microvm.services.api.v1alpha1.ListMicroVMsResponse)
    - [UpdateMicroVMRequest](#microvm.services.api.v1alpha1.UpdateMicroVMRequest)
    - [UpdateMicroVMResponse](#microvm.services.api.v1alpha1.UpdateMicroVMResponse)
  
    - [MicroVM](#microvm.services.api.v1alpha1.MicroVM)
  
- [Scalar Value Types](#scalar-value-types)



<a name="services/microvm/v1alpha1/microvms.proto"></a>
<p align="right"><a href="#top">Top</a></p>

## services/microvm/v1alpha1/microvms.proto



<a name="microvm.services.api.v1alpha1.CreateMicroVMRequest"></a>

### CreateMicroVMRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| microvm | [flintlock.types.MicroVMSpec](#flintlock.types.MicroVMSpec) |  |  |
| metadata | [CreateMicroVMRequest.MetadataEntry](#microvm.services.api.v1alpha1.CreateMicroVMRequest.MetadataEntry) | repeated |  |






<a name="microvm.services.api.v1alpha1.CreateMicroVMRequest.MetadataEntry"></a>

### CreateMicroVMRequest.MetadataEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [google.protobuf.Any](#google.protobuf.Any) |  |  |






<a name="microvm.services.api.v1alpha1.CreateMicroVMResponse"></a>

### CreateMicroVMResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| microvm | [flintlock.types.MicroVMSpec](#flintlock.types.MicroVMSpec) |  |  |






<a name="microvm.services.api.v1alpha1.DeleteMicroVMRequest"></a>

### DeleteMicroVMRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  |  |
| namespace | [string](#string) |  |  |






<a name="microvm.services.api.v1alpha1.GetMicroVMRequest"></a>

### GetMicroVMRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| id | [string](#string) |  |  |
| namespace | [string](#string) |  |  |






<a name="microvm.services.api.v1alpha1.GetMicroVMResponse"></a>

### GetMicroVMResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| microvm | [flintlock.types.MicroVM](#flintlock.types.MicroVM) |  |  |






<a name="microvm.services.api.v1alpha1.ListMessage"></a>

### ListMessage



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| microvm | [flintlock.types.MicroVMSpec](#flintlock.types.MicroVMSpec) |  |  |






<a name="microvm.services.api.v1alpha1.ListMicroVMsRequest"></a>

### ListMicroVMsRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| namespace | [string](#string) |  |  |






<a name="microvm.services.api.v1alpha1.ListMicroVMsResponse"></a>

### ListMicroVMsResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| microvm | [flintlock.types.MicroVM](#flintlock.types.MicroVM) | repeated |  |






<a name="microvm.services.api.v1alpha1.UpdateMicroVMRequest"></a>

### UpdateMicroVMRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| microvm | [flintlock.types.MicroVMSpec](#flintlock.types.MicroVMSpec) |  |  |
| update_mask | [google.protobuf.FieldMask](#google.protobuf.FieldMask) |  |  |






<a name="microvm.services.api.v1alpha1.UpdateMicroVMResponse"></a>

### UpdateMicroVMResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| microvm | [flintlock.types.MicroVMSpec](#flintlock.types.MicroVMSpec) |  |  |





 

 

 


<a name="microvm.services.api.v1alpha1.MicroVM"></a>

### MicroVM
MicroVM providers a service to create and manage the lifecycle of microvms.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| CreateMicroVM | [CreateMicroVMRequest](#microvm.services.api.v1alpha1.CreateMicroVMRequest) | [CreateMicroVMResponse](#microvm.services.api.v1alpha1.CreateMicroVMResponse) |  |
| UpdateMicroVM | [UpdateMicroVMRequest](#microvm.services.api.v1alpha1.UpdateMicroVMRequest) | [UpdateMicroVMResponse](#microvm.services.api.v1alpha1.UpdateMicroVMResponse) |  |
| DeleteMicroVM | [DeleteMicroVMRequest](#microvm.services.api.v1alpha1.DeleteMicroVMRequest) | [.google.protobuf.Empty](#google.protobuf.Empty) |  |
| GetMicroVM | [GetMicroVMRequest](#microvm.services.api.v1alpha1.GetMicroVMRequest) | [GetMicroVMResponse](#microvm.services.api.v1alpha1.GetMicroVMResponse) |  |
| ListMicroVMs | [ListMicroVMsRequest](#microvm.services.api.v1alpha1.ListMicroVMsRequest) | [ListMicroVMsResponse](#microvm.services.api.v1alpha1.ListMicroVMsResponse) |  |
| ListMicroVMsStream | [ListMicroVMsRequest](#microvm.services.api.v1alpha1.ListMicroVMsRequest) | [ListMessage](#microvm.services.api.v1alpha1.ListMessage) stream |  |

 



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

