#!/usr/bin/env bash
# Spikes A and B of issue #1359, see section 9 of
# 0204-snapshot-restore-design.md. Run as root on a host with KVM and
# firecracker v1.16.1 on the PATH, with /srv/spike on XFS with reflink.
set -uo pipefail

W=/srv/spike
RUN=/run/spike
mkdir -p "$W" "$RUN"
cd "$W"

log() { printf '\n===== %s\n' "$*"; }
api() { # api <sock> <method> <path> [json]
	local out code
	out=$(curl -sS --unix-socket "$1" -X "$2" "http://localhost$3" \
		-H 'Content-Type: application/json' ${4:+-d "$4"} -w '\n%{http_code}')
	code=${out##*$'\n'}
	echo "$2 $3 ${4:-} -> HTTP $code ${out%$'\n'*}"
	[[ $code == 2* ]]
}

log "versions"
firecracker --version
firecracker --snapshot-version
uname -r

log "kernel and rootfs"
skopeo copy -q docker://ghcr.io/liquidmetal-dev/firecracker-kernel:5.10-20260929-1349 oci:kimg:k
mkdir -p kroot && for l in kimg/blobs/sha256/*; do tar -xf "$l" -C kroot 2>/dev/null || true; done
KERNEL=$(find kroot -type f -name 'vmlinux*' | head -n1)
echo "kernel: $KERNEL"

skopeo copy -q docker://ghcr.io/liquidmetal-dev/ubuntu:24.04 oci:rimg:r
mkdir -p rroot
for d in $(jq -r '.manifests[0].digest' rimg/index.json | sed 's/sha256://'); do
	for l in $(jq -r '.layers[].digest' "rimg/blobs/sha256/$d" | sed 's/sha256://'); do
		tar -xf "rimg/blobs/sha256/$l" -C rroot
	done
done
printf '#!/bin/sh\nmount -t proc proc /proc\necho spike-guest-up\nwhile true; do sleep 1; done\n' > rroot/spike-init
chmod +x rroot/spike-init
truncate -s 2G rootfs.ext4
mkfs.ext4 -q -d rroot rootfs.ext4

# make_vm <name>: a working directory with volumes/root.img and a sock link.
make_vm() {
	mkdir -p "$W/$1/volumes" "$RUN/$1"
	cp --reflink=auto rootfs.ext4 "$W/$1/volumes/root.img"
	ln -sfn "$RUN/$1" "$W/$1/sock"
}

# start_fc <name>: start firecracker with cwd = $W/<name>; prints pid.
start_fc() {
	rm -f "$RUN/$1.api"
	(cd "$W/$1" && exec firecracker --api-sock "$RUN/$1.api" --id "$1" \
		>"$W/$1.out" 2>&1 </dev/null) &
	local pid=$!
	for _ in $(seq 50); do [[ -S $RUN/$1.api ]] && break; sleep 0.1; done
	echo "$pid"
}

show_vm() { # show_vm <name> <pid>
	echo "--- cwd of $1: $(readlink /proc/$2/cwd)"
	echo "--- fds of $1 (pid $2):"
	ls -l /proc/$2/fd | sed 's/^/    /'
	echo "--- listening unix sockets of pid $2:"
	ss -xlp | grep "pid=$2," | sed 's/^/    /'
	echo "--- $RUN/$1:"
	ls -l "$RUN/$1" | sed 's/^/    /'
}

log "spike A: source X with relative drive and vsock paths"
make_vm X
XPID=$(start_fc X)
X=$RUN/X.api
api $X PUT /boot-source "{\"kernel_image_path\":\"$W/$KERNEL\",\"boot_args\":\"console=ttyS0 reboot=k panic=1 pci=off init=/spike-init\"}"
api $X PUT /drives/root '{"drive_id":"root","path_on_host":"volumes/root.img","is_root_device":true,"is_read_only":false}'
api $X PUT /machine-config '{"vcpu_count":1,"mem_size_mib":256}'
api $X PUT /vsock '{"guest_cid":3,"uds_path":"sock/guest-agent.vsock"}'
api $X PUT /actions '{"action_type":"InstanceStart"}'
sleep 8
grep -m1 spike-guest-up "$W/X.out" || { echo "guest did not print its marker"; tail -n 30 "$W/X.out"; }
show_vm X "$XPID"

log "spike A: snapshot of X"
mkdir -p "$W/snap"
api $X PATCH /vm '{"state":"Paused"}'
echo "--- with sync_snapshot_files:"
if api $X PUT /snapshot/create "{\"snapshot_type\":\"Full\",\"snapshot_path\":\"$W/snap/vmstate\",\"mem_file_path\":\"$W/snap/mem\",\"sync_snapshot_files\":true}"; then
	SYNC_OK=yes
else
	SYNC_OK=no
	echo "--- without sync_snapshot_files:"
	api $X PUT /snapshot/create "{\"snapshot_type\":\"Full\",\"snapshot_path\":\"$W/snap/vmstate\",\"mem_file_path\":\"$W/snap/mem\"}"
fi
echo "sync_snapshot_files accepted: $SYNC_OK"
ls -l "$W/snap"
cp --reflink=auto "$W/X/volumes/root.img" "$W/snap/root.img"
api $X PATCH /vm '{"state":"Resumed"}'
echo "--- vsock path stored in the snapshot (strings):"
strings "$W/snap/vmstate" | grep -E 'guest-agent.vsock|volumes/root' | sed 's/^/    /'

log "spike A: restore Y from a different working directory, X still running"
make_vm Y
cp --reflink=auto "$W/snap/root.img" "$W/Y/volumes/root.img"
YPID=$(start_fc Y)
api $RUN/Y.api PUT /snapshot/load "{\"snapshot_path\":\"$W/snap/vmstate\",\"mem_backend\":{\"backend_type\":\"File\",\"backend_path\":\"$W/snap/mem\"},\"resume_vm\":true}"
sleep 3
api $RUN/Y.api GET /vm/config >/dev/null; api $RUN/Y.api GET / || true
show_vm Y "$YPID"
echo "--- X still running:"; api $X GET / || true
show_vm X "$XPID"

log "spike B: load the same memory file in a second process Z at once"
make_vm Z
cp --reflink=auto "$W/snap/root.img" "$W/Z/volumes/root.img"
ZPID=$(start_fc Z)
api $RUN/Z.api PUT /snapshot/load "{\"snapshot_path\":\"$W/snap/vmstate\",\"mem_backend\":{\"backend_type\":\"File\",\"backend_path\":\"$W/snap/mem\"},\"resume_vm\":true}"
sleep 3
echo "--- Y:"; api $RUN/Y.api GET / || true
echo "--- Z:"; api $RUN/Z.api GET / || true
show_vm Z "$ZPID"
for p in $YPID $ZPID; do
	echo "--- smaps of the memory file mapping in pid $p:"
	awk -v f="$W/snap/mem" '$0 ~ f {s=1; print; next} s && /^[0-9a-f]+-[0-9a-f]+ / {s=0} s && /^(Size|Rss|Pss|Shared_Clean|Shared_Dirty|Private_Clean|Private_Dirty):/ {print "    " $0}' /proc/$p/smaps
done
echo "--- fincore of the memory file:"
fincore "$W/snap/mem" || true
echo "--- memory file size:"; ls -l "$W/snap/mem"

log "done"
kill $XPID $YPID $ZPID 2>/dev/null || true
