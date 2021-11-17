#!/bin/bash

# WARNING: THIS SCRIPT HAS MUTLIPLE PURPOSES.
# TAKE CARE WHEN EDITING.

# This is a scripted version of https://docs.docker.com/storage/storagedriver/device-mapper-driver/#configure-direct-lvm-mode-manually

set -ex

if [[ $(id -u) != 0 ]]; then
  echo "Run this script as root..." >&2
  exit 1
fi

DISK_NAME="$THINPOOL_DISK_NAME"
POOL_NAME_PREFIX="flintlock"

usage() {
cat << EOF
usage: $0 -d DISK_NAME [-p POOL_NAME]

Script to generate release data for eksctl and profiles projects.

OPTIONS:
   -d YEAR	  (Required if \$THINPOOL_DISK_NAME not set) Set the name of the disk to use for thinpool storage
   -p POOL_NAME	  Name of the thinpool to create (default: flintlock)
   -h		  Show this message
EOF
}

while getopts ":hd:p:" OPTION
do
  case $OPTION in
    h)
      usage
      exit
      ;;
    d)
      DISK_NAME=$OPTARG
      ;;
    p)
      POOL_NAME_PREFIX=$OPTARG
      ;;
    ?)
      usage
      exit
      ;;
  esac
done

if [[ -z "$DISK_NAME" ]]; then
  echo "one of \$THINPOOL_DISK_NAME or '-d NAME' must be set"
  exit 1
fi

DISK_PATH="/dev/$DISK_NAME"
PROFILE="/etc/lvm/profile/$POOL_NAME_PREFIX-thinpool.profile"
CROOT=/var/lib/containerd-flintlock
DIR="${CROOT}/snapshotter/devmapper"

echo "will create thinpool $POOL_NAME_PREFIX-thinpool on $DISK_PATH"

apt update
apt install -y thin-provisioning-tools lvm2

if [[ $(pvdisplay) != *"$DISK_PATH"* ]]; then
  pvcreate "$DISK_PATH"
  echo "created physical volume on $DISK_PATH"
fi

if [[ $(vgdisplay) != *"$POOL_NAME_PREFIX"* ]]; then
  vgcreate "$POOL_NAME_PREFIX" "$DISK_PATH"
  echo "created volume group on $DISK_PATH"
fi

if [[ $(lvdisplay) != *"$POOL_NAME_PREFIX"* ]]; then
  lvcreate --wipesignatures y -n thinpool "$POOL_NAME_PREFIX" -l 95%VG
  echo "created logical volume for $POOL_NAME_PREFIX data"

  lvcreate --wipesignatures y -n thinpoolmeta "$POOL_NAME_PREFIX" -l 1%VG
  echo "created logical volume for $POOL_NAME_PREFIX metadata"

  lvconvert -y \
    --zero n \
    -c 512K \
    --thinpool "$POOL_NAME_PREFIX"/thinpool \
    --poolmetadata "$POOL_NAME_PREFIX"/thinpoolmeta
  echo "converted logical volumes to thinpool storage"
fi


if [[ ! -f "$PROFILE" ]]; then
cat <<'EOF' >> "$PROFILE"
activation {
  thin_pool_autoextend_threshold=80
  thin_pool_autoextend_percent=20
}
EOF
echo "written lvm profile to $PROFILE"
fi

if [[ $(lvs) != *"$POOL_NAME_PREFIX"* ]]; then
  lvchange --metadataprofile "$POOL_NAME_PREFIX-thinpool" "$POOL_NAME_PREFIX"/thinpool
  echo "applied lvm profile for $POOL_NAME_PREFIX-thinpool"
fi

try=1
max=5
while [ "$try" -le "$max" ]; do
  echo "checking that lvm profile for $POOL_NAME_PREFIX-thinpool is monitored"
  if [[ $(lvs -o+seg_monitor) == *"not monitored"* ]]; then
    lvchange --monitor y "$POOL_NAME_PREFIX"/thinpool
    ((try=try+1))

    if [[ "$try" -gt "$max" ]]; then
      echo "could not monitor lvm profile"
      exit 1
    fi

    continue
  fi

  break
done

echo "successfully monitored $POOL_NAME_PREFIX-thinpool profile"
echo "thinpool $POOL_NAME_PREFIX-thinpool is ready for use"

cat << EOF
#
# Add this to your config.toml configuration file and restart containerd daemon
#
[plugins]
  [plugins.devmapper]
    pool_name = "${POOL_NAME_PREFIX}-thinpool"
    root_path = "${DIR}"
    base_image_size = "10GB"
    discard_blocks = true
EOF
