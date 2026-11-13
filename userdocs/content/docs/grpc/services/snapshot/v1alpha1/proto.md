---
title: snapshot.services.api.v1alpha1
editURL: https://github.com/liquidmetal-dev/flintlock/edit/main/api/services/snapshot/v1alpha1/snapshots.proto
---
<a name="top"></a>

## Table of Contents

- [services/snapshot/v1alpha1/snapshots.proto](#services_snapshot_v1alpha1_snapshots-proto)
    - [CreateSnapshotRequest](#snapshot-services-api-v1alpha1-CreateSnapshotRequest)
    - [CreateSnapshotRequest.LabelsEntry](#snapshot-services-api-v1alpha1-CreateSnapshotRequest-LabelsEntry)
    - [CreateSnapshotResponse](#snapshot-services-api-v1alpha1-CreateSnapshotResponse)
    - [DeleteSnapshotRequest](#snapshot-services-api-v1alpha1-DeleteSnapshotRequest)
    - [GetSnapshotRequest](#snapshot-services-api-v1alpha1-GetSnapshotRequest)
    - [GetSnapshotResponse](#snapshot-services-api-v1alpha1-GetSnapshotResponse)
    - [ListSnapshotsMessage](#snapshot-services-api-v1alpha1-ListSnapshotsMessage)
    - [ListSnapshotsRequest](#snapshot-services-api-v1alpha1-ListSnapshotsRequest)
    - [ListSnapshotsRequest.LabelsEntry](#snapshot-services-api-v1alpha1-ListSnapshotsRequest-LabelsEntry)
    - [ListSnapshotsResponse](#snapshot-services-api-v1alpha1-ListSnapshotsResponse)
  
    - [After](#snapshot-services-api-v1alpha1-After)
  
    - [SnapshotService](#snapshot-services-api-v1alpha1-SnapshotService)
  
- [Scalar Value Types](#scalar-value-types)



<a name="services_snapshot_v1alpha1_snapshots-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## services/snapshot/v1alpha1/snapshots.proto



<a name="snapshot-services-api-v1alpha1-CreateSnapshotRequest"></a>

### CreateSnapshotRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| source_uid | [string](#string) |  | SourceUID is the UID of the microvm to snapshot. |
| name | [string](#string) | optional | Name is the optional name of the snapshot. It must be unique in the namespace of the source microvm. |
| labels | [CreateSnapshotRequest.LabelsEntry](#snapshot-services-api-v1alpha1-CreateSnapshotRequest-LabelsEntry) | repeated | Labels allows you to include extra data for the snapshot. |
| after | [After](#snapshot-services-api-v1alpha1-After) |  | After selects what happens to the source microvm once the snapshot is taken. The default is RESUME. |






<a name="snapshot-services-api-v1alpha1-CreateSnapshotRequest-LabelsEntry"></a>

### CreateSnapshotRequest.LabelsEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="snapshot-services-api-v1alpha1-CreateSnapshotResponse"></a>

### CreateSnapshotResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| snapshot | [flintlock.types.Snapshot](#flintlock-types-Snapshot) |  |  |






<a name="snapshot-services-api-v1alpha1-DeleteSnapshotRequest"></a>

### DeleteSnapshotRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| ref | [flintlock.types.SnapshotRef](#flintlock-types-SnapshotRef) |  |  |






<a name="snapshot-services-api-v1alpha1-GetSnapshotRequest"></a>

### GetSnapshotRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| ref | [flintlock.types.SnapshotRef](#flintlock-types-SnapshotRef) |  |  |






<a name="snapshot-services-api-v1alpha1-GetSnapshotResponse"></a>

### GetSnapshotResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| snapshot | [flintlock.types.Snapshot](#flintlock-types-Snapshot) |  |  |






<a name="snapshot-services-api-v1alpha1-ListSnapshotsMessage"></a>

### ListSnapshotsMessage



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| snapshot | [flintlock.types.Snapshot](#flintlock-types-Snapshot) |  |  |






<a name="snapshot-services-api-v1alpha1-ListSnapshotsRequest"></a>

### ListSnapshotsRequest



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| namespace | [string](#string) |  | Namespace is the namespace of the snapshots to list. |
| source_uid | [string](#string) | optional | SourceUID optionally limits the list to the snapshots of one microvm. |
| labels | [ListSnapshotsRequest.LabelsEntry](#snapshot-services-api-v1alpha1-ListSnapshotsRequest-LabelsEntry) | repeated | Labels optionally limits the list to the snapshots that have all of these labels. |






<a name="snapshot-services-api-v1alpha1-ListSnapshotsRequest-LabelsEntry"></a>

### ListSnapshotsRequest.LabelsEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |






<a name="snapshot-services-api-v1alpha1-ListSnapshotsResponse"></a>

### ListSnapshotsResponse



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| snapshots | [flintlock.types.Snapshot](#flintlock-types-Snapshot) | repeated |  |





 <!-- end messages -->


<a name="snapshot-services-api-v1alpha1-After"></a>

### After
After selects what happens to the source microvm once the snapshot is taken.

| Name | Number | Description |
| ---- | ------ | ----------- |
| RESUME | 0 | RESUME resumes the source microvm. |
| STOP | 1 | STOP stops the source microvm. It is not supported yet. |


 <!-- end enums -->

 <!-- end HasExtensions -->


<a name="snapshot-services-api-v1alpha1-SnapshotService"></a>

### SnapshotService
SnapshotService provides a service to create and manage snapshots of microvms.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| CreateSnapshot | [CreateSnapshotRequest](#snapshot-services-api-v1alpha1-CreateSnapshotRequest) | [CreateSnapshotResponse](#snapshot-services-api-v1alpha1-CreateSnapshotResponse) |  |
| GetSnapshot | [GetSnapshotRequest](#snapshot-services-api-v1alpha1-GetSnapshotRequest) | [GetSnapshotResponse](#snapshot-services-api-v1alpha1-GetSnapshotResponse) |  |
| ListSnapshots | [ListSnapshotsRequest](#snapshot-services-api-v1alpha1-ListSnapshotsRequest) | [ListSnapshotsResponse](#snapshot-services-api-v1alpha1-ListSnapshotsResponse) |  |
| ListSnapshotsStream | [ListSnapshotsRequest](#snapshot-services-api-v1alpha1-ListSnapshotsRequest) | [ListSnapshotsMessage](#snapshot-services-api-v1alpha1-ListSnapshotsMessage) stream |  |
| DeleteSnapshot | [DeleteSnapshotRequest](#snapshot-services-api-v1alpha1-DeleteSnapshotRequest) | [.google.protobuf.Empty](#google-protobuf-Empty) |  |

 <!-- end services -->



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

