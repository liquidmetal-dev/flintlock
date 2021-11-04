#!/bin/bash

CTRD_ROOT="/var/lib/containerd-dev"
CTRD_STATE="/run/containerd-dev"
CTRD_CFG="/etc/containerd"
DM_ROOT="$CTRD_ROOT/snapshotter/devmapper"

mkdir -p "$DM_ROOT" "$CTRD_STATE" "$CTRD_CFG"
cat > "$CTRD_CFG/config-dev.toml" <<EOF
version = 2
root = "$CTRD_ROOT"
state = "$CTRD_STATE"
[grpc]
  address = "$CTRD_STATE/containerd.sock"
[metrics]
  address = "127.0.0.1:1338"
[plugins]
  [plugins."io.containerd.snapshotter.v1.devmapper"]
    pool_name = "$POOL"
    root_path = "$DM_ROOT"
    base_image_size = "10GB"
    discard_blocks = true
[debug]
  level = "trace"
EOF

mv /usr/local/go/bin/go /usr/local/bin

mkdir -p /root/work && cd /root/work
# TODO make it so that repo/branch etc can be configured for local runs / copy up binary / clone at commit etc
git clone https://github.com/weaveworks/flintlock --depth 1

touch /flintlock_ready
