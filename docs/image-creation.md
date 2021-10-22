# Container Image Creation for MicroVM usage

These are temporary instructions on how to create container images for use by the microVM as a source for:

- Volumes
- Kernel
- Initrd

## Setup

Run the following command to download the Ubuntu Server cloud images:

```shell
hack/scripts/download_cloudimages.sh
```

This downloads the Ubuntu Server Cloud Image files and and processes them. The downloaded files and processed files will be available in `out/images` by default. There are a number of flags that can be used for custimization:

| Flag            |  Description                                                     |
| --------------- | ---------------------------------------------------------------- |
| -o/--output     | Specifies the output folder to use. Defaults to `./out`.         |
| -v/--version    | Specifies the ubuntu version to download. Defaults to `bionic`.  |
| -s/--image-size | Specifies the size of the root fs to create. Defaluts to `10G`.  |

The processed files (i.e. root filesystem, uncompressed kernel, initrd) can be used directly with Firecracker without flintlock.

> As an alternative using the download script you can use [debootstrap](https://wiki.debian.org/Debootstrap) by running `sudo debootstrap bionic ./out/images/mount > /dev/null`. The commands in the following sections may need to be adapted.

## Building a volume container image

1. Run the following to mount the downloaded and processed root filesystem:

```shell
mkdir -p out/images/mount
sudo mount -o loop out/images/bionic/bionic.rootfs ./out/images/mount
```

2. Run the following to create the container image (replacing `myorg/ubuntu-bionic-volume:v0.0.1` with your required container image name/tag):

```shell
sudo tar -C ./out/images/mount -c . | docker import - myorg/ubuntu-bionic-volume:v0.0.1
docker push myorg/ubuntu-bionic-volume:v0.0.1
```

## Building a Kernel/Initrd container image

1. Create a Dockerfile that adds the uncompressed kernel and initrd. For example:

```dockerfile
FROM scratch

COPY vmlinux initrd-generic /
```

2. Use docker build and then push
