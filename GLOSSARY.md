# flintlock

flintlock creates and manages microVMs. It runs each microVM with a VMM
(Firecracker or Cloud Hypervisor) and gives the guest its cloud-init data.

## Language

**Spec metadata**:
The key-value map in a microVM spec that holds the cloud-init data of the
guest, with each value base64-encoded.
_Avoid_: Metadata (alone), instance metadata

**Vendor data**:
The cloud-init document that flintlock or its caller gives in addition to the
user data, under the key `vendor-data`.
_Avoid_: Vendor config

**Default vendor data**:
The empty cloud-config document that flintlock gives a Firecracker guest when
the spec metadata has no vendor data.
_Avoid_: Fallback vendor data, placeholder vendor data

**MMDS**:
The metadata service of Firecracker, from which a Firecracker guest reads its
cloud-init data over the network.
_Avoid_: Metadata server, IMDS
