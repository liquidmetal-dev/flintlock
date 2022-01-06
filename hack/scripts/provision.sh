#!/usr/bin/env bash

# Tool to provision hosts for running flintlock microvms
# './hack/scripts/provision.sh --help' for commands
# or see hack/scripts/README.md for full docs.

set -o pipefail

# general vars
MY_NAME="flintlock $(basename "$0")"
INSTALL_PATH="/usr/local/bin"
OPT_UNATTENDED=false
DEVELOPMENT=false
DEFAULT_VERSION="latest"
DEFAULT_BRANCH="main"

# paths to be set later, put here to be explicit
CONTAINERD_CONFIG_PATH=""
CONTAINERD_ROOT_DIR=""
CONTAINERD_STATE_DIR=""
DEVMAPPER_DIR=""
DEVPOOL_METADATA=""
DEVPOOL_DATA=""

# firecracker
FIRECRACKER_VERSION="${FIRECRACKER:=$DEFAULT_VERSION}"
FIRECRACKER_BIN="firecracker"
FIRECRACKER_RELEASE_BIN="firecracker"
FIRECRACKER_REPO="weaveworks/firecracker"

# containerd
CONTAINERD_VERSION="${CONTAINERD:=$DEFAULT_VERSION}"
CONTAINERD_BIN="containerd"
CONTAINERD_REPO="containerd/containerd"
CONTAINERD_SERVICE_FILE="/etc/systemd/system/containerd.service"

# flintlock
FLINTLOCK_VERSION="${FLINTLOCK:=$DEFAULT_VERSION}"
FLINTLOCK_BIN="flintlockd"
FLINTLOCK_REPO="weaveworks/flintlock"
FLINTLOCK_RELEASE_BIN="flintlockd_amd64"
FLINTLOCKD_SERVICE_FILE="/etc/systemd/system/flintlockd.service"

# thinpool
THINPOOL_PROFILE_PATH="/etc/lvm/profile"
DEFAULT_THINPOOL="flintlock"
DEFAULT_DEV_THINPOOL="flintlock-dev"
DATA_SPARSE_SIZE="100G"
METADATA_SPARSE_SIZE="10G"
# Magic calculations, for more information go and read
# https://www.kernel.org/doc/Documentation/device-mapper/thin-provisioning.txt
SECTORSIZE=512
DATA_BLOCK_SIZE=128
# picked arbitrarily
# If free space on the data device drops below this level then a dm event will
# be triggered which a userspace daemon should catch allowing it to extend the
# pool device.
LOW_WATER_MARK=32768

## HELPER FUNCS
#
#
# Send a green message to stdout, followed by a new line
say() {
	[ -t 1 ] && [ -n "$TERM" ] &&
		echo "$(tput setaf 2)[$MY_NAME]$(tput sgr0) $*" ||
		echo "[$MY_NAME] $*"
}

# Send a green message to stdout, without a trailing new line
say_noln() {
	[ -t 1 ] && [ -n "$TERM" ] &&
		echo -n "$(tput setaf 2)[$MY_NAME]$(tput sgr0) $*" ||
		echo "[$MY_NAME] $*"
}

# Send a red message to stdout, followed by a new line
say_err() {
	[ -t 2 ] && [ -n "$TERM" ] &&
		echo -e "$(tput setaf 1)[$MY_NAME] $*$(tput sgr0)" 1>&2 ||
		echo -e "[$MY_NAME] $*" 1>&2
}

# Send a yellow message to stdout, followed by a new line
say_warn() {
	[ -t 1 ] && [ -n "$TERM" ] &&
		echo "$(tput setaf 3)[$MY_NAME] $*$(tput sgr0)" ||
		echo "[$MY_NAME] $*"
}

# Send a yellow message to stdout, without a trailing new line
say_warn_noln() {
	[ -t 1 ] && [ -n "$TERM" ] &&
		echo -n "$(tput setaf 3)[$MY_NAME] $*$(tput sgr0)" ||
		echo "[$MY_NAME] $*"
}

# Exit with an error message and (optional) code
# Usage: die [-c <error code>] <error message>
die() {
	code=1
	[[ "$1" = "-c" ]] && {
		code="$2"
		shift 2
	}
	say_err "$@"
	exit "$code"
}

# Exit with an error message if the last exit code is not 0
ok_or_die() {
	code=$?
	[[ $code -eq 0 ]] || die -c $code "$@"
}

# Check if /dev/kvm exists. Exit if it doesn't.
ensure_kvm() {
	[[ -c /dev/kvm ]] || die "/dev/kvm not found. Required for virtualisation. Aborting."
}

# Pause whatever is going on to ask for user confirmation
# Can pass optional custom message and options
get_user_confirmation() {
	# Skip if running unattended
	[[ "$OPT_UNATTENDED" = true ]] && return 0

	# Fail if STDIN is not a terminal (there's no user to confirm anything)
	[[ -t 0 ]] || return 1

	# Otherwise, ask the user
	msg=$([ -n "$1" ] && echo -n "$1" || echo -n "Continue? (y/n) ")
	yes=$([ -n "$2" ] && echo -n "$2" || echo -n "y")
	say_warn_noln "$msg"
	# shellcheck disable=SC2162
	read c && [ "$c" = "$yes" ] && return 0
	return 1
}

## BUILDER FUNCS
#
#
# Returns URL to latest release
build_release_url() {
	local repo_name="$1"
	echo "https://api.github.com/repos/$repo_name/releases/latest"
}

# Returns containerd release binary name for linux-amd64
# If/when we need to support others, we can ammend
build_containerd_release_bin_name() {
	local tag=${1//v/} # remove the 'v' from arg $1
	echo "containerd-$tag-linux-amd64.tar.gz"
}

# Returns the desired binary download url for a repo, tag and binary
build_download_url() {
	local repo_name="$1"
	local tag="$2"
	local bin="$3"
	echo "https://github.com/$repo_name/releases/download/$tag/$bin"
}

# Returns the URL to a raw github file
build_raw_url() {
	local repo_name="$1"
	local file_name="$2"
	echo "https://raw.githubusercontent.com/$repo_name/$DEFAULT_BRANCH/$file_name"
}

# Sets various global variables for state paths
build_containerd_paths() {
	local tag=""

	if [[ "$DEVELOPMENT" == "true" ]]; then
		tag="-dev"
	fi

	CONTAINERD_CONFIG_PATH="/etc/containerd/config$tag.toml"
	CONTAINERD_ROOT_DIR="/var/lib/containerd$tag"
	CONTAINERD_STATE_DIR="/run/containerd$tag"
	DEVMAPPER_DIR="$CONTAINERD_ROOT_DIR/snapshotter/devmapper"
	DEVPOOL_METADATA="$DEVMAPPER_DIR/metadata"
	DEVPOOL_DATA="$DEVMAPPER_DIR/data"
}

## DOER FUNCS
#
#
# Returns the tag associated with a "latest" release
latest_release_tag() {
	# shellcheck disable=SC2155
	local latest_url=$(build_release_url "$1")
	# shellcheck disable=SC2155
	local url=$(curl -sL "$latest_url" | awk -F'"' '/tag_name/ {printf $4}')
	echo "$url"
}

# Returns the tag associated with a "latest" pre-release (pre-releases do not show
# up in the API as used in the above latest_release_tag func)
latest_pre_release_tag() {
	local repo_name="$1"
	tag=$(git ls-remote --tags --sort="v:refname" "https://github.com/$repo_name" |
		tail -n 1 | sed 's/.*\///; s/\^{}//')
	echo "$tag"
}

# Install the untarred binary attached to a release to /usr/local/bin
install_release_bin() {
	local download_url="$1"
	local dest_file="$2"
	wget -q "$download_url" -O "$INSTALL_PATH/$dest_file" || die "failed to download release for $dest_file"
	chmod +x "$INSTALL_PATH/$dest_file"
}

# Install and untar the tarred binary attached to a release to /usr/local/bin
install_release_tar() {
	local download_url="$1"
	local dest_path="$2"
	curl -sL "$download_url" | tar xz -C "$dest_path"
}

# Set and create the correct state dirs
prepare_dirs() {
	build_containerd_paths
	make_containerd_dirs
}

# Download the given service file from the given repo
fetch_service_file() {
	local repo="$1"
	local service="$2"
	local dest="$3"
	# shellcheck disable=SC2155
	local url=$(build_raw_url "$repo" "$service")
	curl -o "$dest" -sL "$url" || die "failed to download $service"
	chmod 0664 "$dest"
	systemctl daemon-reload
}

# Enable and start the given systemd service
start_service() {
	local service="$1"
	systemctl enable "$service" &>/dev/null || die "failed to enable $service service"
	systemctl start "$service" || die "failed to start $service service"
}

## FIRECRACKER
#
#
# Fetch and install the firecracker binary
install_firecracker() {
	local tag="$1"
	say "Installing firecracker version $tag to $INSTALL_PATH"

	if [[ "$tag" == "$DEFAULT_VERSION" ]]; then
		tag=$(latest_release_tag "$FIRECRACKER_REPO")
	fi

	url=$(build_download_url "$FIRECRACKER_REPO" "$tag" "$FIRECRACKER_RELEASE_BIN")
	install_release_bin "$url" "$FIRECRACKER_BIN" || die "could not install firecracker"

	"$FIRECRACKER_BIN" --version &>/dev/null
	ok_or_die "firecracker version $tag not installed"

	say "Firecracker version $tag successfully installed"
}

## FLINTLOCK
#
#
do_all_flintlock() {
	local version="$1"
	local address="$2"
	install_flintlockd "$version"
	start_flintlockd_service "$address"
}

# Fetch and install the flintlockd binary at the specified version
install_flintlockd() {
	local tag="$1"
	say "Installing flintlockd version $tag to $INSTALL_PATH"

	if [[ "$tag" == "$DEFAULT_VERSION" ]]; then
		tag=$(latest_pre_release_tag "$FLINTLOCK_REPO")
	fi

	url=$(build_download_url "$FLINTLOCK_REPO" "$tag" "$FLINTLOCK_RELEASE_BIN")
	install_release_bin "$url" "$FLINTLOCK_BIN"

	"$FLINTLOCK_BIN" version &>/dev/null
	ok_or_die "Flintlockd version $tag not installed"

	say "Flintlockd version $tag successfully installed"
}

# Fetch and start the flintlock systemd service
start_flintlockd_service() {
	local address="$1"

	say "Starting flintlockd service"

	service=$(basename "$FLINTLOCKD_SERVICE_FILE")
	fetch_service_file "$FLINTLOCK_REPO" "$service" "$FLINTLOCKD_SERVICE_FILE"
	edit_service_file "$address"
	start_service "$FLINTLOCK_BIN"

	say "Flintlockd running at $address:9090"
}

# This is a temporary work-around func while I sort out a config file for
# flintlock
# Don't look too closely at it, it is not long for this world :p
edit_service_file() {
	local address="$1"

	parent=$(ip route show | awk '/default/ {print $5}')
	sed -i "s/PARENT_IFACE/$parent/" "$FLINTLOCKD_SERVICE_FILE"
	socket="$CONTAINERD_STATE_DIR/containerd.sock"
	sed -i "s|\(socket=\).*\(\\)|\1$socket \\\\\2|g" "$FLINTLOCKD_SERVICE_FILE"

	if [[ -z "$address" ]]; then
		address=$(lookup_address)
	fi
	sed -i "s/ADDRESS/$address/" "$FLINTLOCKD_SERVICE_FILE"
}

# Returns the internal address of the host
lookup_address() {
	ip route show | awk '/scope link/ {print $9}' | grep -E '^(192\.168|10\.|172\.1[6789]\.|172\.2[0-9]\.|172\.3[01]\.)'
}

## CONTAINERD
#
#
do_all_containerd() {
	local version="$1"
	local thinpool="$2"
	install_containerd "$version"
	write_containerd_config "$thinpool"
	start_containerd_service
}

# Fetch and install the containerd binary
install_containerd() {
	local tag="$1"
	say "Installing containerd version $tag to $INSTALL_PATH"

	if [[ "$version" == "$DEFAULT_VERSION" ]]; then
		tag=$(latest_release_tag "$CONTAINERD_REPO")
	fi

	bin=$(build_containerd_release_bin_name "$tag")
	url=$(build_download_url "$CONTAINERD_REPO" "$tag" "$bin")
	install_release_tar "$url" "$(dirname $INSTALL_PATH)" || die "could not install containerd"

	"$CONTAINERD_BIN" --version &>/dev/null
	ok_or_die "Containerd version $tag not installed"

	say "Containerd version $tag successfully installed"
}

# Prepare the containerd state dirs
make_containerd_dirs() {
	local dirs=("$DEVMAPPER_DIR" "$CONTAINERD_STATE_DIR" "$(dirname $CONTAINERD_CONFIG_PATH)")
	for d in "${dirs[@]}"; do
		say "Creating containerd directory $d"

		mkdir -p "$d" || die "Failed to make containerd directory $d"
	done

	say "All containerd directories created"
}

# Write out the containerd config file
write_containerd_config() {
	local thinpool="$1"

	say "Writing containerd config to $CONTAINERD_CONFIG_PATH"

	cat <<EOF >"$CONTAINERD_CONFIG_PATH"
version = 2

root = "$CONTAINERD_ROOT_DIR"
state = "$CONTAINERD_STATE_DIR"

[grpc]
  address = "$CONTAINERD_STATE_DIR/containerd.sock"

[metrics]
  address = "127.0.0.1:1338"

[plugins]
  [plugins."io.containerd.snapshotter.v1.devmapper"]
    pool_name = "$thinpool-thinpool"
    root_path = "$DEVMAPPER_DIR"
    base_image_size = "10GB"
    discard_blocks = true

[debug]
  level = "trace"
EOF

	say "Containerd config saved"
}

# Start the containerd systemd service
start_containerd_service() {
	say "Starting containerd service"

	service=$(basename "$CONTAINERD_SERVICE_FILE")
	fetch_service_file "$CONTAINERD_REPO" "$service" "$CONTAINERD_SERVICE_FILE"

	sed -i "s|ExecStart=.*|& --config $CONTAINERD_CONFIG_PATH|" "$CONTAINERD_SERVICE_FILE"

	start_service "$CONTAINERD_BIN"

	say "Containerd running"
}

## DIRECT_LVM
#
#
do_all_direct_lvm() {
	local disk="$1"
	local thinpool="$2"

	say "Setting up direct_lvm thinpool $thinpool"

	if [[ -z "$disk" ]]; then
		say_warn "WARNING: -d/--disk has not been set. If you continue, the" \
			"script will attempt to detect a free disk for formatting. Any data" \
			"will be lost."
		get_user_confirmation "Are you sure you wish to continue? (y/n) " || die "Aborted."

		disk=$(find_free_disk || die "Could not detect free disk")
	fi

	disk_name=$(basename "$disk")
	local disk_path="/dev/$disk_name"

	say "Will use $disk_path for direct-lvm thinpool $thinpool"
	say_warn "All existing data on $disk_path will be overwritten."
	get_user_confirmation || die "Aborted."

	create_physical_volume "$disk_path"
	create_volume_group "$disk_path" "$thinpool"
	create_logical_volume "$thinpool"
	apply_lvm_profile "$thinpool"
	monitor_lvm_profile "$thinpool" || die "failed to monitor lvm profile"

	say "Thinpool $thinpool-thinpool is ready for use"
}

# Naively find a spare block device which is not in use
# This is really unsafe as it only looks at a couple of things to decide anything
# It is a much better idea to use the --disk flag and pass in something you
# know will be available
find_free_disk() {
	disks=("$(lsblk -o NAME,TYPE | awk '/disk/ {print $1}')")

	# shellcheck disable=SC2068
	for d in ${disks[@]}; do
		if ! is_mounted "$d" && ! is_partitioned "$d"; then
			echo "$d" && return 0
		fi
	done

	return 1
}

# Check whether given device is mounted
is_mounted() {
	local device_name="$1"
	findmnt -rno TARGET "/dev/$device_name" >/dev/null
}

# Check whether given device is partitioned
is_partitioned() {
	local device_name="$1"
	sfdisk -d "/dev/$device_name" &>/dev/null
}

# Create a physical volume on the given device
create_physical_volume() {
	local disk_path="$1"

	# if already exists, do nothing
	if [[ $(pvdisplay 2>/dev/null) != *"$disk_path"* ]]; then
		pvcreate -q "$disk_path" || die "failed to create physical volume on $disk_path"
		say "Created physical volume on $disk_path"
		return 0
	fi

	say "Physical volume on $disk_path already exists"
}

# Create a volume group on the given device for the thinpool
create_volume_group() {
	local disk_path="$1"
	local thinpool="$2"

	# if already exists, do nothing
	if [[ $(vgdisplay 2>/dev/null) != *"$thinpool"* ]]; then
		vgcreate -q "$thinpool" "$disk_path" || die "failed to create volume group on $disk_path"
		say "Created volume group on $disk_path"
		return 0
	fi

	say "Volume group on $disk_path already exists"
}

# Format the volume for thinpool storage
create_logical_volume() {
	local volume_group="$1"

	# if already exists, do nothing
	if [[ $(lvdisplay 2>/dev/null) != *"$volume_group"* ]]; then
		lvcreate -q --wipesignatures y -n thinpool "$volume_group" -l 95%VG || die "Failed to create logical volume for thinpool data"
		say "Created logical volume for $volume_group thinpool data"

		lvcreate -q --wipesignatures y -n thinpoolmeta "$thinpool" -l 1%VG || die "Failed to create logical volume for thinpool metadata"
		say "Created logical volume for $volume_group thinpool metadata"

		lvconvert -q -y \
			--zero n \
			-c 512K \
			--thinpool "$volume_group"/thinpool \
			--poolmetadata "$volume_group"/thinpoolmeta || die "Failed to convert logical volumes to thinpool storage"
		say "Converted logical volumes to thinpool storage"

		return 0
	fi

	say "Logical volume for $volume_group thinpool already exists"
}

# Create and apply the lvm profile for the thinpool
apply_lvm_profile() {
	local thinpool="$1"
	local profile="$THINPOOL_PROFILE_PATH/$thinpool-thinpool.profile"

	if [[ ! -f "$profile" ]]; then
		cat <<'EOF' >>"$profile"
activation {
thin_pool_autoextend_threshold=80
thin_pool_autoextend_percent=20
}
EOF
		say "Written lvm profile to $profile"
	fi

	# if already exists, do nothing
	if [[ $(lvs 2>/dev/null) != *"$thinpool"* ]]; then
		lvchange -q --metadataprofile "$thinpool-thinpool" "$thinpool"/thinpool || die "Could not apply lvm profile $profile"
		say "Applied lvm profile for $thinpool-thinpool"
		return 0
	fi

	say "LVM profile for $thinpool-thinpool already applied"
}

# Try 5 times to ensure the lvm profile is monitored
monitor_lvm_profile() {
	local thinpool="$1"

	for _ in $(seq 5); do
		if [[ $(lvs -o+seg_monitor 2>/dev/null) != *"not monitored"* ]]; then
			say "Successfully monitored ${thinpool}-thinpool profile"
			return
		fi
		lvchange --monitor y "${thinpool}/thinpool"
	done

	die -c 1
}

## DEVPOOL
#
#
do_all_devpool() {
	local thinpool="$1-thinpool"

	say "Will create loop-back thinpool $thinpool"

	create_sparse_file "$DEVPOOL_DATA" "$DATA_SPARSE_SIZE"
	create_sparse_file "$DEVPOOL_METADATA" "$METADATA_SPARSE_SIZE"

	say "Associating loop devices with sparse files"
	datadev=$(associate_loop_device "$DEVPOOL_DATA")
	metadev=$(associate_loop_device "$DEVPOOL_METADATA")
	say "Loop devices $datadev and $metadev associated"

	create_dev_thinpool "$thinpool" "$datadev" "$metadev"

	say "Dev thinpool creation complete"
}

# Create the a sparse file which will be used to back a loop device
create_sparse_file() {
	local file="$1"
	local size="$2"

	say "Creating sparse file $file of size $size"
	if [[ ! -f "$file" ]]; then
		touch "$file"
		truncate -s "$size" "$file" || die "Failed to create sparse file $file"
	fi

	say "Sparse file $file created"
}

# Assign a loop device to the given sparse file
associate_loop_device() {
	local sparse_file="$1"

	device="$(losetup --output NAME --noheadings --associated "$sparse_file")"
	if [[ -z "$device" ]]; then
		device=$(losetup --find --show "$sparse_file" || die "Failed to associate loop device with $sparse_file")
	fi

	echo "$device"
}

# Create the thinpool with the loop devices if it does not already exist
create_dev_thinpool() {
	local thinpool="$1"
	local datadev="$2"
	local metadev="$3"

	say "Creating thinpool $thinpool with devices $datadev and $metadev"

	datasize="$(blockdev --getsize64 -q "$datadev")"
	length_sectors=$(bc <<<"$datasize/$SECTORSIZE")
	thinp_table="0 $length_sectors thin-pool $metadev $datadev $DATA_BLOCK_SIZE $LOW_WATER_MARK 1 skip_block_zeroing"

	if ! dmsetup reload "$thinpool" --table "$thinp_table" 2>/dev/null; then
		dmsetup create "$thinpool" --table "$thinp_table" || die "failed to create dev thinpool $thinpool"
	fi

	say "Thinpool $thinpool created"
}

## COMMANDS
#
#
cmd_all() {
	local skip_apt=false
	local disk=""
	local fl_address=""
	local thinpool="$DEFAULT_THINPOOL"
	local fc_version="$FIRECRACKER_VERSION"
	local fl_version="$FLINTLOCK_VERSION"
	local ctrd_version="$CONTAINERD_VERSION"

	while [ $# -gt 0 ]; do
		case "$1" in
		"-h" | "--help")
			cmd_all_help
			exit 1
			;;
		"-y" | "--unattended")
			OPT_UNATTENDED=true
			;;
		"-d" | "--disk")
			shift
			disk="$1"
			;;
		"-t" | "--thinpool")
			shift
			thinpool="$1"
			;;
		"-a" | "--grpc-address")
			shift
			fl_address="$1"
			;;
		"-s" | "--skip-apt")
			skip_apt=true
			;;
		"--dev")
			DEVELOPMENT=true
			;;
		*)
			die "Unknown argument: $1. Please use --help for help."
			;;
		esac
		shift
	done

	say "$(date -u +'%F %H:%M:%S %Z'): Provisioning host $(hostname)"
	say "The following subcommands will be performed:" \
		"apt, firecracker, containerd, flintlock, direct_lvm|devpool"

	ensure_kvm

	if [[ "$skip_apt" == false ]]; then
		cmd_apt
	fi

	prepare_dirs

	# if the env is a dev one, then we don't want to use a real disk
	# and we want to tag all state dirs with 'dev'
	if [[ "$DEVELOPMENT" == false ]]; then
		set_thinpool="${DEFAULT_THINPOOL:=$thinpool}"
		do_all_direct_lvm "$disk" "$set_thinpool"
	else
		set_thinpool="${DEFAULT_DEV_THINPOOL:=$thinpool}"
		do_all_devpool "$set_thinpool"
	fi

	install_firecracker "$fc_version"
	do_all_containerd "$ctrd_version" "$set_thinpool"
	do_all_flintlock "$fl_version" "$fl_address"

	say "$(date -u +'%F %H:%M:%S %Z'): Host $(hostname) provisioned"
}

cmd_apt() {
	say "Installing required apt packages"
	apt update
	apt install -qq -y \
		thin-provisioning-tools \
		lvm2 \
		git \
		curl \
		wget
	say "Packages installed"
}

cmd_firecracker() {
	local version="$FIRECRACKER_VERSION"

	while [ $# -gt 0 ]; do
		case "$1" in
		"-h" | "--help")
			cmd_firecracker_help
			exit 1
			;;
		"-v" | "--version")
			shift
			version="$1"
			;;
		*)
			die "Unknown argument: $1. Please use --help for help."
			;;
		esac
		shift
	done

	install_firecracker "$version"
}

cmd_containerd() {
	local version="$CONTAINERD_VERSION"
	local thinpool="$DEFAULT_THINPOOL"

	while [ $# -gt 0 ]; do
		case "$1" in
		"-h" | "--help")
			cmd_containerd_help
			exit 1
			;;
		"-v" | "--version")
			shift
			version="$1"
			;;
		"-t" | "--thinpool")
			shift
			thinpool="$1"
			;;
		"--dev")
			DEVELOPMENT=true
			;;
		*)
			die "Unknown argument: $1. Please use --help for help."
			;;
		esac
		shift
	done

	prepare_dirs
	do_all_containerd "$version" "$thinpool"
}

cmd_flintlock() {
	local version="$FLINTLOCK_VERSION"
	local address=""

	while [ $# -gt 0 ]; do
		case "$1" in
		"-h" | "--help")
			cmd_flintlock_help
			exit 1
			;;
		"-v" | "--version")
			shift
			version="$1"
			;;
		"-a" | "--grpc-address")
			shift
			address="$1"
			;;
		"--dev")
			DEVELOPMENT=true
			;;
		*)
			die "Unknown argument: $1. Please use --help for help."
			;;
		esac
		shift
	done

	prepare_dirs
	do_all_flintlock "$version" "$address"
}

cmd_direct_lvm() {
	local thinpool="$DEFAULT_THINPOOL"
	local skip_apt=false

	local disk=""
	while [ $# -gt 0 ]; do
		case "$1" in
		"-h" | "--help")
			cmd_direct_lvm_help
			exit 1
			;;
		"-y" | "--unattended")
			OPT_UNATTENDED=true
			;;
		"-d" | "--disk")
			shift
			disk="$1"
			;;
		"-t" | "--thinpool")
			shift
			thinpool="$1"
			;;
		"-s" | "--skip-apt")
			skip_apt=true
			;;
		*)
			die "Unknown argument: $1. Please use --help for help."
			;;
		esac
		shift
	done

	if [[ "$skip_apt" == false ]]; then
		cmd_apt
	fi

	do_all_direct_lvm "$disk" "$thinpool"
	say_warn "remember to set pool_name to $thinpool-thinpool in your containerd config"
}

cmd_devpool() {
	local thinpool="$DEFAULT_DEV_THINPOOL"

	local disk=""
	while [ $# -gt 0 ]; do
		case "$1" in
		"-h" | "--help")
			cmd_devpool_help
			exit 1
			;;
		"-t" | "--thinpool")
			shift
			thinpool="$1"
			;;
		*)
			die "Unknown argument: $1. Please use --help for help."
			;;
		esac
		shift
	done

	DEVELOPMENT=true
	prepare_dirs
	do_all_devpool "$thinpool"
	say_warn "remember to set pool_name to $thinpool-thinpool in your containerd config"
}

## COMMAND HELP FUNCS
#
#
cmd_apt_help() {
	cat <<EOF
  apt                    Install all apt packages required by flintlock

EOF
}

cmd_all_help() {
	cat <<EOF
  all                    Complete setup for production ready host. Component versions
                         can be configured by setting the FLINTLOCK, CONTAINERD and FIRECRACKER
			 environment variables.
    OPTIONS:
      -y                 Autoapprove all prompts (danger)
      --skip-apt, -s     Skip installation of apt packages
      --thinpool, -t     Name of thinpool to create (default: flintlock or flintlock-dev)
      --disk, -d         Name blank unpartioned disk to use for direct lvm thinpool (ignored if --dev set)
      --grpc-address, -a Address on which to start the GRPC server (default: local ipv4 address)
      --dev              Set up development environment. Loop thinpools will be created.

EOF
}

cmd_firecracker_help() {
	cat <<EOF
  firecracker            Install firecracker from feature branch
    OPTIONS:
      --version, -v      Version to install (default: latest)

EOF
}

cmd_containerd_help() {
	cat <<EOF
  containerd             Install, configure and start containerd service
    OPTIONS:
      --version, -v      Version to install (default: latest)
      --thinpool, -t     Name of thinpool to include in config toml (default: flintlock-thinpool)
      --dev              Set up development environment. Containerd will keep state under 'dev' tagged paths.

EOF
}

cmd_flintlock_help() {
	cat <<EOF
  flintlock              Install and start flintlockd service (note: will not succeed without containerd)
    OPTIONS:
      --version, -v      Version to install (default: latest)
      --grpc-address, -a Address on which to start the GRPC server (default: local ipv4 address)
      --dev              Assumes containerd has been provisioned in a dev environment

EOF
}

cmd_direct_lvm_help() {
	cat <<EOF
  direct_lvm             Set up direct_lvm thinpool
    OPTIONS:
      -y                 Autoapprove all prompts (danger)
      --thinpool, -t     Name of thinpool to create (default: flintlock)
      --disk, -d         Name blank unpartioned disk to use for direct lvm thinpool
      --skip-apt, -s     Skip installation of apt packages

EOF
}

cmd_devpool_help() {
	cat <<EOF
  devpool                Set up loop device thinpool (development environments)
    OPTIONS:
      --thinpool, -t     Name of thinpool to create (default: flintlock-dev)

EOF
}

cmd_help() {
	cat <<EOF
usage: $0 <COMMAND> <OPTIONS>

Script to provision hosts for running flintlock microvms

COMMANDS:

EOF

	cmd_all_help
	cmd_apt_help
	cmd_firecracker_help
	cmd_containerd_help
	cmd_flintlock_help
	cmd_direct_lvm_help
	cmd_devpool_help
}

## LET'S DO THIS THING
#
#
main() {
	if [ $# = 0 ]; then
		die "No command provided. Please use \`$0 help\` for help."
	fi

	# Parse main command line args.
	#
	while [ $# -gt 0 ]; do
		case "$1" in
		-h | --help)
			cmd_help
			exit 1
			;;
		-*)
			die "Unknown arg: $1. Please use \`$0 help\` for help."
			;;
		*)
			break
			;;
		esac
		shift
	done

	if [[ $(id -u) != 0 ]]; then
		die "Run this script as root..." >&2
	fi

	# $1 is now a command name. Check if it is a valid command and, if so,
	# run it.
	#
	declare -f "cmd_$1" >/dev/null
	ok_or_die "Unknown command: $1. Please use \`$0 help\` for help."

	cmd=cmd_$1
	shift

	# $@ is now a list of command-specific args
	#
	$cmd "$@"
}

main "$@"
