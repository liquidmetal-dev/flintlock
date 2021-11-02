---
sidebar_position: 1
---

# Configure network

:::info
Flintlock and flintlock tests are only compatible with Linux. We recommend that
non-linux users provision a [Linux VM][vagrant] in which to work.
:::

[vagrant]: ../extras/use-vagrant

If you are using wired connection, you can skip this and jump straight to the
"Containerd" section. With wireless adapter, macvtap has some issues. The easy
workaround is to use a bridge and tap devices instead.

You can use the default kvm network, in this case, skip to "Create and connect
tap device" and use `default`. We recommend using a dedicated network to avoid
interference from other kvm machines or processes like IP or MAC address
conflict.

### Install packages and start `libvirtd`

```bash
sudo apt install qemu qemu-kvm libvirt-clients libvirt-daemon-system virtinst bridge-utils

sudo systemctl enable libvirtd
sudo systemctl start libvirtd
```

## Create kvm network

Create the `flintlock.xml` file (feel free to change the IP range):

```xml
<network>
  <name>flintlock</name>
  <forward mode='nat'>
    <nat>
      <port start='1024' end='65535'/>
    </nat>
  </forward>
  <bridge name='flkbr0' stp='on' delay='0'/>
  <ip address='192.168.100.1' netmask='255.255.255.0'>
    <dhcp>
      <range start='192.168.100.2' end='192.168.100.254'/>
    </dhcp>
  </ip>
</network>
```

Define, start and set autostart on the `flintlock` network:

```
sudo virsh net-define flintlock.xml
sudo virsh net-start flintlock
sudo virsh net-autostart flintlock
```

Now you should see the network in the network list:

```
virsh net-list
 Name       State    Autostart   Persistent
---------------------------------------------
 default    active   yes         yes
 flintlock   active   yes         yes
```

## Create and connect tap device

```bash
tapName=tap0
bridge=flkbr0
sudo ip tuntap add ${tapName} mode tap
sudo ip link set ${tapName} master ${bridge} up
```

Check with `ip link ls`.

You can add a function into your bashrc/zshrc:

```bash
function vir-new-tap() {
  tapName=${1:=tap0}
  bridge=${2:=flkbr0}

  sudo ip tuntap add ${tapName} mode tap
  sudo ip link set ${tapName} master ${bridge} up
}
```

You can check the DHCP leases with `virsh`:

```bash
sudo virsh net-dhcp-leases default
```
