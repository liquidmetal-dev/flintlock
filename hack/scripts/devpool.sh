#!/bin/bash

# WARNING: THIS SCRIPT HAS MUTLIPLE PURPOSES.
# TAKE CARE WHEN EDITING.

set -ex

if [[ $(id -u) != 0 ]]; then
  echo "Run this script as root..." >&2
  exit 1
fi

# That's where our stuff will live.
CROOT=/var/lib/containerd-dev
# This is the name of the thinpool.
POOL="${1:-dev-thinpool}"
set +u
# This is the tag which will be appended to the loop device volumes
VOL_TAG=""
if [[ -n "$2" ]]; then
  VOL_TAG="-$2"
fi
set -u
# These are some useful vars for useful things
DIR="${CROOT}/snapshotter/devmapper"
META="${DIR}/metadata$VOL_TAG"
DATA="${DIR}/data$VOL_TAG"

mkdir -p "${DIR}"

# Create "data" file/volume if it's not there and set it's size to 100G.
if [[ ! -f "${DATA}" ]]; then
touch "${DATA}"
truncate -s 100G "${DATA}"
fi

# Create "metadata" file/volume if it's not there and set it's size to 2G.
if [[ ! -f "${META}" ]]; then
touch "${META}"
truncate -s 10G "${META}"
fi

# Find/associate a loop device with our data volume.
DATADEV="$(losetup --output NAME --noheadings --associated ${DATA})"
if [[ -z "${DATADEV}" ]]; then
    DATADEV="$(losetup --find --show ${DATA})"
fi

# Find/associate a loop device with our metadata volume.
METADEV="$(losetup --output NAME --noheadings --associated ${META})"
if [[ -z "${METADEV}" ]]; then
    METADEV="$(losetup --find --show ${META})"
fi

# Magic calculations, for more information go and read
# https://www.kernel.org/doc/Documentation/device-mapper/thin-provisioning.txt
SECTORSIZE=512
DATASIZE="$(blockdev --getsize64 -q ${DATADEV})"
LENGTH_SECTORS=$(bc <<< "${DATASIZE}/${SECTORSIZE}")
DATA_BLOCK_SIZE=128
# picked arbitrarily
# If free space on the data device drops below this level then a dm event will
# be triggered which a userspace daemon should catch allowing it to extend the
# pool device.
LOW_WATER_MARK=32768

THINP_TABLE="0 ${LENGTH_SECTORS} thin-pool ${METADEV} ${DATADEV} ${DATA_BLOCK_SIZE} ${LOW_WATER_MARK} 1 skip_block_zeroing"
echo "${THINP_TABLE}"

# If thinpool does not exist yet, create one.
if ! $(dmsetup reload "${POOL}" --table "${THINP_TABLE}"); then
     dmsetup create "${POOL}" --table "${THINP_TABLE}"
fi

cat << EOF
#
# Add this to your config.toml configuration file and restart containerd daemon
#
[plugins]
  [plugins.devmapper]
    pool_name = "${POOL}"
    root_path = "${DIR}"
    base_image_size = "10GB"
    discard_blocks = true
EOF
