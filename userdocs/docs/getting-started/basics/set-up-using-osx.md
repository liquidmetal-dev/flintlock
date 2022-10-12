---
sidebar_position: 6
---

# Set up OSX

To set up an environment that can run flintlock, capmvm and other tools,
you can use `Vagrantfile` provided in this repository.

First, install [Vagrant][vagrant].

:::info
To give vagrant a virtualization provider you could use [VirtualBox][virtualbox]
for example.
:::

Second, clone the repository:

```bash
git clone https://github.com/weaveworks-liquidmetal/flintlock
cd flintlock
```

Then run `vagrant up` to get the machine started.

Once the machine is up and running, execute `vagrant ssh`. While you're in
there optionally run `sudo su`, because everything will need sudo anyway.

Next, install some necessary tools. Currently, the required tools are:

- [docker][docker]
- [kind][kind]
- [clusterctl][clusterctl]
- [kubectl][kubectl]

Once all of these are installed, you can continue with executing the
[getting started guide][getting-started].

[virtualbox]: https://www.virtualbox.org/wiki/Downloads
[vagrant]: https://www.vagrantup.com/downloads
[docker]: https://docs.docker.com/engine/install/ubuntu/
[kind]: https://kind.sigs.k8s.io/docs/user/quick-start/
[clusterctl]: https://cluster-api.sigs.k8s.io/user/quick-start.html
[kubectl]: https://kubernetes.io/docs/tasks/tools/install-kubectl-linux/
[getting-started]: ./configuring-network
