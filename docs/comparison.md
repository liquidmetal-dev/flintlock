# Firecracker & Cloud Hypervisor Comparisson

## Actions

| Action        | Firecracker | Cloud Hypervisor |
| ------------- | ------ | ------------- |
| Start         | Yes    | Yes (boot)    |
| Flush Metrics | Yes    | ?             |
| CtrlAltDel    | Yes    | ?             |
| Pause         | Yes    | Yes           |
| Resume        | Yes    | Yes           |
| Delete        | No     | Yes           |
| Shutdown      | ?      | Yes           |
| Reboot        | No     | Yes           |
| Resize        | No     | Yes           |


## Machine

| Data           | Firecracker | Cloud Hypervisor |
| --------------- | -------- | ----- |
| Boot Args       | Yes      | Yes   |
| Initrd          | Yes      | Yes   |
| Kernel Image    | Yes      | Yes   |
| CPU Template    | Yes      | ?     |
| Hyper Threading | Yes      | Yes   |
| Mem Size        | Yes      | Yes   |
| Track Dirty Pg  | Yes      | ?     |
| vcpu            | Yes      | Yes   |
| max vcps        | No       | Yes   |
| cpu topology    | No       | Yes   |
| hotplug size    | No       | Yes   |
| Shared Memory   | No       | Yes   |
| Huge Pages      | No       | Yes   |


## Drive

| Data | Firecracker| Cloud Hypervisor |
| ------------- | ------- | ----- |
| Is Root       | Yes     | ?     |
| Is Read Only  | Yes     | Yes   |
| Cache Type    | Yes     | ?     |
| Partition ID  | Yes     | ?     |
| Host Path     | Yes     | Yes   |
| RX Rate Limit | Yes     | Yes   |
| TX rate Limit | Yes     | ?     |
| Direct        | ?       | Yes   |
| iommu         | ?       | Yes   |
| Queues (size) | ?       | Yes   |
| vhost         | ?       | Yes   |


## Network Interface

| Data | Firecracker | Cloud Hypervisor |
| -------- | -------- | -------- |
| Query MMDS    | yes     | ?     |
| Guest MAC     | Yes     | Yes   |
| Host Dev      | Yes     | Yes   |
| Iface id      | Yes     | Yes   |
|     - TAP     | Yes     | Yes   |
|     - macvtap | Yes     | Yes   |
| RX Rate Limit | Yes     | Yes   |
| TX rate Limit | Yes     | Yes   |
| iommu         | ?       | Yes   |
| Queues (size) | ?       | Yes   |
| vhost         | ?       | Yes   |
| IP Address    | ?       | Yes   |
| Netmask       | ?       | Yes   |


## Additional Features

| Feature | Firecracker | Cloud Hypervisor |
| -------- | -------- | -------- |
| Metadata Service | Yes     | ?     |
| Logging          | Yes     | ?     |
| Metrics          | Yes     | Yes   |
| Snapshot         | Yes     | Yes   |
| vsock            | Yes     | Yes   |
| Baloon Device    | Yes     | Yes   |
| Random Num gen   | ?       | Yes   |
| fs (dir share)   | ?       | Yes   |
| Serial           | Yes     | Yes   |
| Console          | ?       | Yes   |
| Devices???       | ?       | Yes   |
| SGX              | ?       | Yes   |
| Numa             | Yes     | Yes   |
| pmem             | ?       | Yes   |
| Live migration   | ?       | Yes   |
