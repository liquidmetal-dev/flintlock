# Protocol Documentation
<a name="top"></a>

## Table of Contents

- [services/microvmsshproxy/v1alpha1/microvmsshproxy.proto](#services_microvmsshproxy_v1alpha1_microvmsshproxy-proto)
    - [SSHProxyRequest](#microvmsshproxy-services-api-v1alpha1-SSHProxyRequest)
    - [SSHProxyResponse](#microvmsshproxy-services-api-v1alpha1-SSHProxyResponse)
  
    - [MicroVMSSHProxy](#microvmsshproxy-services-api-v1alpha1-MicroVMSSHProxy)
  
- [Scalar Value Types](#scalar-value-types)



<a name="services_microvmsshproxy_v1alpha1_microvmsshproxy-proto"></a>
<p align="right"><a href="#top">Top</a></p>

## services/microvmsshproxy/v1alpha1/microvmsshproxy.proto



<a name="microvmsshproxy-services-api-v1alpha1-SSHProxyRequest"></a>

### SSHProxyRequest
SSHProxyRequest is one message in the client-&gt;server stream. The first
message on a stream must carry uid; all following messages carry data
bytes to forward toward the guest&#39;s sshd.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| uid | [string](#string) |  | uid is the microvm identifier, as used by MicroVM.GetMicroVM/DeleteMicroVM. |
| data | [bytes](#bytes) |  |  |






<a name="microvmsshproxy-services-api-v1alpha1-SSHProxyResponse"></a>

### SSHProxyResponse
SSHProxyResponse carries raw bytes received from the guest&#39;s sshd.


| Field | Type | Label | Description |
| ----- | ---- | ----- | ----------- |
| data | [bytes](#bytes) |  |  |





 

 

 


<a name="microvmsshproxy-services-api-v1alpha1-MicroVMSSHProxy"></a>

### MicroVMSSHProxy
MicroVMSSHProxy tunnels a raw byte stream to a microvm&#39;s guest-agent ssh
port, which itself proxies straight to the guest&#39;s local sshd - the agent
performs no authentication of its own; sshd owns auth, PTY and SFTP. It
requires the target microvm to have been created with AllowGuestAgent set,
to be in the created state, and for the server to have been started with
ssh proxying enabled.

| Method Name | Request Type | Response Type | Description |
| ----------- | ------------ | ------------- | ------------|
| SSHProxy | [SSHProxyRequest](#microvmsshproxy-services-api-v1alpha1-SSHProxyRequest) stream | [SSHProxyResponse](#microvmsshproxy-services-api-v1alpha1-SSHProxyResponse) stream |  |

 



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

