# Protocol Documentation
<a name="top"></a>

## Table of Contents

- [services/microvmexec/v1alpha1/microvmexec.proto](#services_microvmexec_v1alpha1_microvmexec-proto)
    - [ExecCommandRequest](#microvmexec-services-api-v1alpha1-ExecCommandRequest)
    - [ExecCommandResponse](#microvmexec-services-api-v1alpha1-ExecCommandResponse)
    - [ExecStart](#microvmexec-services-api-v1alpha1-ExecStart)
    - [ExecStart.EnvEntry](#microvmexec-services-api-v1alpha1-ExecStart-EnvEntry)
  
    - [MicroVMExec](#microvmexec-services-api-v1alpha1-MicroVMExec)
  
- [Scalar Value Types](#scalar-value-types)



<a name="services_microvmexec_v1alpha1_microvmexec-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## services/microvmexec/v1alpha1/microvmexec.proto



<a name="microvmexec-services-api-v1alpha1-ExecCommandRequest"></a>

### ExecCommandRequest
ExecCommandRequest is one message in the client-&gt;server stream. The first
message on a stream must carry start; all following messages carry stdin
(only meaningful if start.has_stdin was set) or stdin_eof.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| start | [ExecStart](#microvmexec-services-api-v1alpha1-ExecStart) |  |  |
| stdin | [bytes](#bytes) |  |  |
| stdin_eof | [bool](#bool) |  |  |






<a name="microvmexec-services-api-v1alpha1-ExecCommandResponse"></a>

### ExecCommandResponse
ExecCommandResponse is one message in the server-&gt;client stream. Only
exit_code terminates the stream; error may be sent (typically followed by
exit_code) without ending the exchange.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| stdout | [bytes](#bytes) |  |  |
| stderr | [bytes](#bytes) |  |  |
| exit_code | [int32](#int32) |  |  |
| error | [string](#string) |  |  |






<a name="microvmexec-services-api-v1alpha1-ExecStart"></a>

### ExecStart
ExecStart describes the microvm to target and the command to run, mirroring
the guest-agent&#39;s own exec request fields.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| uid | [string](#string) |  | uid is the microvm identifier, as used by MicroVM.GetMicroVM/DeleteMicroVM. |
| cmd | [string](#string) |  |  |
| args | [string](#string) | repeated |  |
| cwd | [string](#string) |  | cwd is the working directory; empty means the guest-agent&#39;s cwd. |
| env | [ExecStart.EnvEntry](#microvmexec-services-api-v1alpha1-ExecStart-EnvEntry) | repeated | env entries are added to (and override) the guest-agent&#39;s environment. |
| shell | [bool](#bool) |  | shell runs &#34;sh -c &lt;cmd &#43; args joined&gt;&#34; instead of a direct exec. |
| user | [string](#string) |  | user, if set, runs the command as that guest system user. |
| timeout_seconds | [int32](#int32) |  | timeout_seconds bounds the run; 0 means unbounded. |
| has_stdin | [bool](#bool) |  | has_stdin tells the server to expect streamed stdin messages. |






<a name="microvmexec-services-api-v1alpha1-ExecStart-EnvEntry"></a>

### ExecStart.EnvEntry



| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| key | [string](#string) |  |  |
| value | [string](#string) |  |  |





 

 

 


<a name="microvmexec-services-api-v1alpha1-MicroVMExec"></a>

### MicroVMExec
MicroVMExec runs commands inside a microvm&#39;s guest-agent over vsock,
streaming stdin in and stdout/stderr/exit back out. It requires the target
microvm to have been created with AllowGuestAgent set, to be in the
created state, and for the server to have been started with exec enabled.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| ExecCommand | [ExecCommandRequest](#microvmexec-services-api-v1alpha1-ExecCommandRequest) stream | [ExecCommandResponse](#microvmexec-services-api-v1alpha1-ExecCommandResponse) stream |  |

 



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

