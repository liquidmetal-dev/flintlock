#!/bin/bash -ex

THINPOOL="dev-thinpool-e2e"
LOOP_TAG="e2e"
CTRD_ROOT="/var/lib/containerd-dev"
CTRD_STATE="/run/containerd-dev"
CTRD_CFG="/etc/containerd"
DM_ROOT="$CTRD_ROOT/snapshotter/devmapper"

cleanup() {
    dmsetup ls | awk -v pool="$THINPOOL" '$0 ~ pool {print $1}' | xargs -I {} dmsetup remove {} --force
    losetup | awk -v loop="$LOOP_TAG" '$0 ~ loop {print $1}' | xargs losetup -d
}

trap 'true' SIGINT SIGTERM

/tmp/devpool.sh "$THINPOOL" "$LOOP_TAG"

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
    pool_name = "$THINPOOL"
    root_path = "$DM_ROOT"
    base_image_size = "10GB"
    discard_blocks = true
[debug]
  level = "trace"
EOF

exec /bin/bash -c "${@}" &

wait $!

cleanup
