#!/usr/bin/env bash
# Spikes A and B of issue #1359, see section 9 of
# 0204-snapshot-restore-design.md. Run as root on a host with KVM and
# firecracker v1.16.1 on the PATH, with /srv/spike on XFS with reflink.
# The script stops at the first failed step or check, and prints PASS at
# the end when every check holds.
set -euo pipefail

W=/srv/spike
RUN=/run/spike
mkdir -p "$W" "$RUN"
cd "$W"

log() { printf '\n===== %s\n' "$*"; }
die() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }
api() { # api <sock> <method> <path> [json]; fails on a status other than 2xx
	local out code
	out=$(curl -sS --unix-socket "$1" -X "$2" "http://localhost$3" \
		-H 'Content-Type: application/json' ${4:+-d "$4"} -w '\n%{http_code}')
	code=${out##*$'\n'}
	echo "$2 $3 ${4:-} -> HTTP $code ${out%$'\n'*}"
	[[ $code == 2* ]]
}
running() { # running <sock>: the VMM reports the Running state
	api "$1" GET / | grep -q '"state":"Running"' || die "$1 is not running"
}

log "versions"
firecracker --version
firecracker --snapshot-version
uname -r

log "kernel and rootfs"
skopeo copy -q docker://ghcr.io/liquidmetal-dev/firecracker-kernel:5.10-20260929-1349 oci:kimg:k
mkdir -p kroot
for l in kimg/blobs/sha256/*; do tar -xf "$l" -C kroot 2>/dev/null || true; done
KERNEL=$(find kroot -type f -name 'vmlinux*' | head -n1)
[[ -n $KERNEL ]] || die "no vmlinux in the kernel image"
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

# start_fc <name>: start firecracker with cwd = $W/<name>; prints the pid.
start_fc() {
	rm -f "$RUN/$1.api"
	(cd "$W/$1" && exec firecracker --api-sock "$RUN/$1.api" --id "$1" "${@:2}" \
		>"$W/$1.out" 2>&1 </dev/null) &
	local pid=$!
	for _ in $(seq 50); do [[ -S $RUN/$1.api ]] && break; sleep 0.1; done
	[[ -S $RUN/$1.api ]] || die "firecracker $1 did not open its API socket"
	echo "$pid"
}

show_vm() { # show_vm <name> <pid>
	echo "--- cwd of $1: $(readlink /proc/$2/cwd)"
	echo "--- files of $1 (pid $2):"
	ls -l /proc/$2/fd | grep -v anon_inode | sed 's/^/    /'
	echo "--- listening unix sockets of pid $2:"
	ss -xlp | grep "pid=$2," | sed 's/^/    /' || true
	echo "--- $RUN/$1:"
	ls -l "$RUN/$1" | sed 's/^/    /'
}

# own_files <name> <pid>: the process opens its own disk and listens on the
# vsock socket in its own socket directory.
own_files() {
	ls -l /proc/$2/fd | grep -q -- "-> $W/$1/volumes/root.img\$" ||
		die "$1 does not open $W/$1/volumes/root.img"
	[[ -S $RUN/$1/guest-agent.vsock ]] || die "no vsock socket in $RUN/$1"
	ss -xlp | grep "pid=$2," | grep -q 'sock/guest-agent.vsock' ||
		die "$1 does not listen on sock/guest-agent.vsock"
	echo "check: $1 has its own disk and vsock socket"
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
for _ in $(seq 30); do grep -q spike-guest-up "$W/X.out" && break; sleep 1; done
grep -q spike-guest-up "$W/X.out" || { tail -n 30 "$W/X.out"; die "the guest of X did not boot"; }
echo "check: the guest of X booted"
running $X
show_vm X "$XPID"
own_files X "$XPID"

log "spike A: snapshot of X"
mkdir -p "$W/snap"
api $X PATCH /vm '{"state":"Paused"}'
echo "--- with sync_snapshot_files (a 400 here is a result, not a failure):"
if api $X PUT /snapshot/create "{\"snapshot_type\":\"Full\",\"snapshot_path\":\"$W/snap/vmstate\",\"mem_file_path\":\"$W/snap/mem\",\"sync_snapshot_files\":true}"; then
	SYNC_OK=yes
else
	SYNC_OK=no
	echo "--- without sync_snapshot_files:"
	api $X PUT /snapshot/create "{\"snapshot_type\":\"Full\",\"snapshot_path\":\"$W/snap/vmstate\",\"mem_file_path\":\"$W/snap/mem\"}"
fi
echo "result: sync_snapshot_files accepted: $SYNC_OK"
[[ -s $W/snap/vmstate && -s $W/snap/mem ]] || die "the snapshot files are missing or empty"
ls -l "$W/snap"
cp --reflink=auto "$W/X/volumes/root.img" "$W/snap/root.img"
api $X PATCH /vm '{"state":"Resumed"}'
echo "--- paths stored in the snapshot (strings):"
strings "$W/snap/vmstate" | grep -E 'guest-agent.vsock|volumes/root' | sed 's/^/    /'
strings "$W/snap/vmstate" | grep -qx 'sock/guest-agent.vsock' || die "the snapshot does not keep the relative vsock path"
strings "$W/snap/vmstate" | grep -qx 'volumes/root.img' || die "the snapshot does not keep the relative drive path"

log "spike A: restore Y from a different working directory, X still running"
make_vm Y
cp --reflink=auto "$W/snap/root.img" "$W/Y/volumes/root.img"
# Y starts as the flintlock provider restores (#1362): the log and metrics
# flags on the command line, load without resume, MMDS put, then resume.
touch "$W/Y.log" "$W/Y.metrics"
YPID=$(start_fc Y --boot-timer --log-path "$W/Y.log" --level Debug --show-level --show-log-origin --metrics-path "$W/Y.metrics")
api $RUN/Y.api PUT /snapshot/load "{\"snapshot_path\":\"$W/snap/vmstate\",\"mem_backend\":{\"backend_type\":\"File\",\"backend_path\":\"$W/snap/mem\"},\"resume_vm\":false}"
if api $RUN/Y.api PUT /mmds '{"restore":{"generation":"1"}}'; then echo "result: PUT /mmds accepted"; else echo "result: PUT /mmds refused (the source has no MMDS config)"; fi
api $RUN/Y.api PATCH /vm '{"state":"Resumed"}'
sleep 3
running $RUN/Y.api
[[ -s $W/Y.log ]] || die "firecracker wrote no log for Y"
for _ in $(seq 70); do [[ -s $W/Y.metrics ]] && break; sleep 1; done
[[ -s $W/Y.metrics ]] || die "firecracker wrote no metrics for Y in 70s"
echo "check: the restore flags give a log and metrics for Y"
echo "--- head of Y.log:"; head -n 5 "$W/Y.log" | sed "s/^/    /"
show_vm Y "$YPID"
own_files Y "$YPID"
running $X
own_files X "$XPID"

log "spike B: load the same memory file in a second process Z at once"
make_vm Z
cp --reflink=auto "$W/snap/root.img" "$W/Z/volumes/root.img"
ZPID=$(start_fc Z)
api $RUN/Z.api PUT /snapshot/load "{\"snapshot_path\":\"$W/snap/vmstate\",\"mem_backend\":{\"backend_type\":\"File\",\"backend_path\":\"$W/snap/mem\"},\"resume_vm\":true}"
sleep 3
running $RUN/Y.api
running $RUN/Z.api
show_vm Z "$ZPID"
own_files Z "$ZPID"

# smaps_field <pid> <field>: the value in kB of a field of the mapping of mem.
smaps_field() {
	awk -v f="$W/snap/mem" -v k="$2:" '$0 ~ f {s=1; next}
		s && /^[0-9a-f]+-[0-9a-f]+ / {s=0} s && $1 == k {print $2; exit}' /proc/$1/smaps
}
for p in $YPID $ZPID; do
	echo "--- smaps of the memory file mapping in pid $p:"
	for k in Size Rss Pss Shared_Clean Shared_Dirty Private_Clean Private_Dirty; do
		printf '    %-14s %s kB\n' "$k" "$(smaps_field "$p" "$k")"
	done
	shared=$(smaps_field "$p" Shared_Clean)
	[[ ${shared:-0} -gt 0 ]] || die "pid $p shares no page of the memory file"
done
echo "check: Y and Z share pages of the memory file"
echo "--- fincore of the memory file:"
if command -v fincore >/dev/null; then fincore "$W/snap/mem"; else echo "fincore not installed"; fi

log "PASS"
kill "$XPID" "$YPID" "$ZPID"
