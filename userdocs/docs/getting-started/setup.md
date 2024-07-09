---
title: Environment setup
---

In this tutorial we will configure a "flintlock host" machine. This can be any machine,
and for now we will assume that you are working from a personal computer.

:::info
This tutorial can be run on **Linux** and **macOS**. HOWEVER, `flintlock` can only
work on Linux. There are extra instructions for OSX users below.
:::

We have tested this tutorial on Ubuntu `20.04` and `22.04`, but any current linux
distribution should do.

This tutorial will set up a dev environment, useful for developing flintlock or
just testing things out. For production environment instructions, see [this page][prod].

## Clone the repo

```bash
git clone https://github.com/liquidmetal-dev/flintlock
cd !$
```

Linux users can now carry on to the next page, mac users: stick around.

## MacOS

In order to complete this tutorial you will need a Linux machine. You can either
get a small server from a Cloud Provider (I believe most have free tiers), or
you can use a virtual machine.

Here we will use [Vagrant][vagrant] and [VirtualBox][virtualbox].

Inside the `flintlock` repo, start the machine:

```bash
vagrant up
```

This may take some time. Once the command has completed, SSH into your new virtual
machine:

```bash
vagrant ssh
```

You will need complete the rest of the tutorial **from within this VM**.

Once you are finished with the tutorial, you can delete the VM with `vagrant destroy`.

[virtualbox]: https://www.virtualbox.org/wiki/Downloads
[vagrant]: https://www.vagrantup.com/downloads
[prod]: /docs/guides/production
