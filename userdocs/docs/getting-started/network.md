---
title: Configure the network
---

# Configure the network

:::tip
If you are using a wired connection, you can skip this and jump straight to the
["Containerd"][containerd] section.

With a wired connection, `flintlock` will be able to create `macvtap` devices for
its MicroVMs. This needs to be enabled in your kernel: check with `modprobe macvlan`.
If this returns non-zero, either install the extra headers or config, or continue
with this page to use `tap` interfaces instead.
:::

If your machine is not on ethernet right now, you will need to set up a local bridge (virtual router).

We recommend using a dedicated network to avoid interference from other kvm machines
or things like IP or MAC address conflict.

## Install packages and start `libvirtd`

```bash
sudo apt install qemu qemu-kvm libvirt-clients libvirt-daemon-system virtinst bridge-utils

sudo systemctl enable libvirtd
sudo systemctl start libvirtd
```

## Create kvm network

Create a file with the network config. Feel free to change the IP range if it
conflicts with any existing network config you have:

```bash
BRIDGE=flbr0
cat << EOF >flintlock-net.xml
<network>
  <name>flintlock</name>
  <forward mode='nat'>
    <nat>
      <port start='1024' end='65535'/>
    </nat>
  </forward>
  <bridge name="$BRIDGE" stp='on' delay='0'/>
  <ip address='192.168.100.1' netmask='255.255.255.0'>
    <dhcp>
      <range start='192.168.100.10' end='192.168.100.254'/>
    </dhcp>
  </ip>
</network>
EOF
```

Define and start the new network:

```bash
sudo virsh net-define flintlock.xml
sudo virsh net-start flintlock
```

If you wish, you can also set it to autostart on boot:

```bash
sudo virsh net-autostart flintlock
```

You should see the network in the network list:

```bash
virsh net-list
 Name       State    Autostart   Persistent
---------------------------------------------
 default    active   yes         yes
 flintlock   active   yes         yes
```

## Create and connect tap device

Create a new `tap` (port) from the bridge to your machine's default interface.

```bash
TAPNAME=tap0

sudo ip tuntap add ${TAPNAME} mode tap
sudo ip link set ${TAPNAME} master ${BRIDGE} up
```

Check with `ip link ls`.

When MicroVMs are created, they will come up in this virtual network and will
request an IP from the DHCP server configured in the network file above.
You will be able to check the DHCP leases with `virsh`:

```bash
sudo virsh net-dhcp-leases default
```

[containerd]: /docs/getting-started/containerd
