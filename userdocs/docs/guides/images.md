---
title: MicroVM images
---

MicroVMs receive kernel binaries and Operating System volumes from container images.
This means that users can easily create and publish their own on Dockerhub.

Compatible images are published as part of the [Liquid Metal][lm] project.

## Supported images

**Kernel**:
- `ghcr.io/weaveworks-liquidmetal/flintlock-kernel:5.10.77`
- `ghcr.io/weaveworks-liquidmetal/flintlock-kernel:4.19.215`

**OS**:

_The tags here refer to the version of Kubernetes._
_The base OS is Ubuntu `20.04`._
_Note that these will attempt to start a `kubelet` on boot._

- `ghcr.io/weaveworks-liquidmetal/capmvm-kubernetes:1.23.5`
- `ghcr.io/weaveworks-liquidmetal/capmvm-kubernetes:1.22.8`
- `ghcr.io/weaveworks-liquidmetal/capmvm-kubernetes:1.22.3`
- `ghcr.io/weaveworks-liquidmetal/capmvm-kubernetes:1.21.8`

## Experimental images

:::warning
These images are not guaranteed to work.
:::

**Kernel**:
- `ghcr.io/weaveworks-liquidmetal/flintlock-kernel-arm:5.10.77`
- `ghcr.io/weaveworks-liquidmetal/flintlock-kernel-arm:4.19.215`

**OS**:

- `ghcr.io/weaveworks-liquidmetal/capmvm-kubernetes-arm:1.23.5`
- `ghcr.io/weaveworks-liquidmetal/capmvm-kubernetes-arm:1.22.8`
- `ghcr.io/weaveworks-liquidmetal/capmvm-kubernetes-arm:1.22.3`
- `ghcr.io/weaveworks-liquidmetal/capmvm-kubernetes-arm:1.21.8`

## Build your own

You can build your own images and supply them in your CreateMicroVM requests.

Our image builder can be found [here][image-builder] if you would like to use it as a base.

:::info
Note that `firecracker` only documents support for `5.10` and `4.19` kernels.
:::

If you'd prefer more bare-bone images, here are some broken down steps for creating
images for volumes, kernels and `initrd`.

### Setup

Run the following command to download the Ubuntu Server cloud images:

```bash
./hack/scripts/download_cloudimages.sh
```

This downloads the Ubuntu Server Cloud Image files and and processes them.
The downloaded files and processed files will be available in `out/images` by default.
There are a number of flags that can be used for custimization:

| Flag            |  Description                                                     |
| --------------- | ---------------------------------------------------------------- |
| -o/--output     | Specifies the output folder to use. Defaults to `./out`.         |
| -v/--version    | Specifies the ubuntu version to download. Defaults to `bionic`.  |
| -s/--image-size | Specifies the size of the root fs to create. Defaluts to `10G`.  |

The processed files (i.e. root filesystem, uncompressed kernel, initrd) can be used directly with Firecracker without flintlock.

:::info
As an alternative using the download script you can use [debootstrap][db]
by running `sudo debootstrap bionic ./out/images/mount > /dev/null`.
The commands in the following sections may need to be adapted.
:::

### Building a volume container image

1. Run the following to mount the downloaded and processed root filesystem:

  ```shell
  mkdir -p out/images/mount
  sudo mount -o loop out/images/bionic/bionic.rootfs ./out/images/mount
  ```

1. Run the following to create the container image (replacing `myorg/ubuntu-bionic-volume:v0.0.1` with your required container image name/tag):

  ```shell
  sudo tar -C ./out/images/mount -c . | docker import - myorg/ubuntu-bionic-volume:v0.0.1
  docker push myorg/ubuntu-bionic-volume:v0.0.1
  ```

### Building a Kernel/Initrd container image

We recommend using [Firecracker's kernel config][fc] if you are building anew.

1. Create a Dockerfile that adds the uncompressed kernel and initrd. For example:

  ```dockerfile
  FROM scratch

  COPY vmlinux initrd-generic /
  ```

1. Use docker build and then push

[image-builder]: https://github.com/weaveworks-liquidmetal/image-builder
[lm]: https://weaveworks-liquidmetal.github.io/site/
[db]: https://wiki.debian.org/Debootstrap
[fc]: https://github.com/firecracker-microvm/firecracker/tree/main/resources/guest_configs
