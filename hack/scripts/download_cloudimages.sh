#!/bin/bash

# This is based on work of Alvaro Hernandez, see: https://gitlab.com/ongresinc/blog-posts-src/-/blob/master/202012-firecracker_cloud_image_automation/03-download_generate_image.sh

OUT_FOLDER="out"
UBUNTU_VERSION="bionic"
IMAGE_SIZE="10G"

function download() {
	echo "Downloading $2..."

	curl -s -o "$1" "$2"
}

function download_if_not_present() {
	[ -f "$1" ] || download "$1" "$2"
}

function generate_image() {
	root_fs="$1"
	size="$2"
	src="$3"

	echo "Generating $root_fs..."

	truncate -s "$size" "$root_fs"
	mkfs.ext4 "$root_fs" > /dev/null 2>&1

	local tmppath=/tmp/.$RANDOM-$RANDOM
	mkdir $tmppath
	sudo mount "$root_fs" -o loop $tmppath
	sudo tar -xf "$src" --directory $tmppath
	sudo umount $tmppath
	rmdir $tmppath
}

function extract_vmlinux() {
	kernel="$1"
	src="$2"

	echo "Extracting vmlinux to $kernel..."

	local extract_linux=/tmp/.$RANDOM-$RANDOM
	curl -s -o $extract_linux https://raw.githubusercontent.com/torvalds/linux/master/scripts/extract-vmlinux
	chmod +x $extract_linux
	$extract_linux "$src" > "$kernel"
	rm $extract_linux
}


for arg in "$@"
do
    case $arg in
        -o=*|--output=*)
        OUT_FOLDER="${arg#*=}"
        shift
        ;;
        -v=*|--version=*)
        UBUNTU_VERSION="${arg#*=}"
        shift
        ;;
        -s=*|--image-size=*)
        IMAGE_SIZE="${arg#*=}"
        shift
        ;;
        *)
        shift
        ;;
    esac
done 

if [ "$OUT_FOLDER" == "" ]; then
    echo "You must supply an output folder using -o. For example -o=out"
    exit 10
fi

if [ "$UBUNTU_VERSION" == "" ]; then
    echo "You must supply an ubuntu version using -v. For example -v=bionic"
    exit 10
fi

if [ "$IMAGE_SIZE" == "" ]; then
    echo "You must supply an image size -s. For example -s=10G"
    exit 10
fi


IMAGE_ROOTFS=$OUT_FOLDER/images/$UBUNTU_VERSION/$UBUNTU_VERSION.rootfs
KERNEL_IMAGE=$OUT_FOLDER/images/$UBUNTU_VERSION/$UBUNTU_VERSION.vmlinux
INITRD=$OUT_FOLDER/images/$UBUNTU_VERSION/$UBUNTU_VERSION.initrd

image_tar=$UBUNTU_VERSION-server-cloudimg-amd64-root.tar.xz
kernel=$UBUNTU_VERSION-server-cloudimg-amd64-vmlinuz-generic
initrd=$UBUNTU_VERSION-server-cloudimg-amd64-initrd-generic

DOWNLOADED_ROOTFS="$OUT_FOLDER/images/$UBUNTU_VERSION/download/$image_tar"
DOWNLOADED_KERNEL="$OUT_FOLDER/images/$UBUNTU_VERSION/download/$kernel"
DOWNLOADED_INITRD="$OUT_FOLDER/images/$UBUNTU_VERSION/download/$initrd"


# Download components
mkdir -p "$OUT_FOLDER/images/$UBUNTU_VERSION/download"

download_if_not_present \
	"$DOWNLOADED_ROOTFS" \
	"https://cloud-images.ubuntu.com/$UBUNTU_VERSION/current/$image_tar"


download_if_not_present \
	"$DOWNLOADED_KERNEL" \
	"https://cloud-images.ubuntu.com/$UBUNTU_VERSION/current/unpacked/$kernel"


download_if_not_present \
	"$DOWNLOADED_INITRD" \
	"https://cloud-images.ubuntu.com/$UBUNTU_VERSION/current/unpacked/$initrd"


# Generate image, kernel 
[ -f "$IMAGE_ROOTFS" ] || generate_image "$IMAGE_ROOTFS" "$IMAGE_SIZE" "$DOWNLOADED_ROOTFS"
[ -f "$INITRD" ] || cp "$DOWNLOADED_INITRD" "$INITRD"
[ -f "$KERNEL_IMAGE" ] || extract_vmlinux "$KERNEL_IMAGE" "$DOWNLOADED_KERNEL"

echo "Use $IMAGE_ROOTFS for your microvm's root filesystem"
echo "Use $KERNEL_IMAGE for your microvm's kernel"
echo "Use $INITRD for your microvm's initial ramdisk"
