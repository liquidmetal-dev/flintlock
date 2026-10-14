#!/usr/bin/env bash

# Build a static linux/amd64 mkfs.erofs from a pinned erofs-utils tag.
#
# Usage: build_erofs_utils.sh [--docker] [<version>] [<output dir>]
#
#   <version>     erofs-utils version to build (default: 1.9.4)
#   <output dir>  where to write the binary and its sha256 (default: ./bin/erofs-utils)
#   --docker      run the build inside a linux/amd64 Debian container. Use this on a
#                 host without the build tools, or on an arm64 host.
#
# The output files are:
#
#   <output dir>/mkfs.erofs-<version>-linux-amd64
#   <output dir>/mkfs.erofs-<version>-linux-amd64.sha256
#
# The release workflow attaches both files to the flintlock release.
# flintlock-provision erofs-utils downloads them by these names.
#
# The build pins the tag to a commit so that a moved tag fails the build.
# Compression libraries, fuse and every other optional dependency are
# disabled: the overlay base build (profile erofs-v1) does not use them, and
# a build without them has no inputs beyond the erofs-utils source and libc.
#
# Native build dependencies (Debian/Ubuntu): git build-essential autoconf
# automake libtool pkg-config.

set -euo pipefail

DEFAULT_VERSION="1.9.4"
EROFS_UTILS_GIT="${EROFS_UTILS_GIT:-https://github.com/erofs/erofs-utils.git}"
BUILD_IMAGE="${BUILD_IMAGE:-debian:bookworm-slim}"

# Commit each supported tag points to. Checked after the clone.
pinned_commit() {
  case "$1" in
    1.9.4) echo "f36cadb5c563995ab3aa8572a60ed6b721b9557d" ;;
    *)
      echo "no pinned commit for erofs-utils $1; add it to $(basename "$0")" >&2
      return 1
      ;;
  esac
}

usage() {
  sed -n '3,/^$/p' "$0" | sed 's/^# \{0,1\}//' >&2
  exit 1
}

use_docker=0
if [[ "${1:-}" == "--docker" ]]; then
  use_docker=1
  shift
fi

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
fi

version="${1:-$DEFAULT_VERSION}"
out_dir="${2:-$PWD/bin/erofs-utils}"
asset="mkfs.erofs-${version}-linux-amd64"

commit="$(pinned_commit "$version")"

mkdir -p "$out_dir"
out_dir="$(cd "$out_dir" && pwd)"

if [[ "$use_docker" == 1 ]]; then
  script="$(cd "$(dirname "$0")" && pwd)/$(basename "$0")"
  docker run --rm --platform linux/amd64 \
    -v "$script:/build_erofs_utils.sh:ro" \
    -v "$out_dir:/out" \
    -e "EROFS_UTILS_GIT=$EROFS_UTILS_GIT" \
    "$BUILD_IMAGE" \
    sh -euc 'apt-get update -qq >/dev/null && \
      DEBIAN_FRONTEND=noninteractive apt-get install -y -qq --no-install-recommends \
        git ca-certificates build-essential autoconf automake libtool pkg-config >/dev/null && \
      /build_erofs_utils.sh "$1" /out && chown "$2" /out/mkfs.erofs-*' -- "$version" "$(id -u):$(id -g)"
  exit 0
fi

arch="$(uname -m)"
if [[ "$arch" != "x86_64" ]]; then
  echo "this script builds linux/amd64 only; host is $arch (use --docker)" >&2
  exit 1
fi

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

echo "cloning erofs-utils v${version} from ${EROFS_UTILS_GIT}"
git -c advice.detachedHead=false clone -q --depth 1 --branch "v${version}" "$EROFS_UTILS_GIT" "$work/src"

head="$(git -C "$work/src" rev-parse HEAD^{commit})"
if [[ "$head" != "$commit" ]]; then
  echo "tag v${version} points to ${head}, expected ${commit}" >&2
  exit 1
fi

cd "$work/src"

./autogen.sh >/dev/null

# Only libc is linked. Every optional library is off, so the configure
# result does not depend on what the build host has installed.
./configure -q \
  --disable-shared --enable-static \
  --disable-lz4 --disable-lzma \
  --without-zlib --without-libdeflate --without-libzstd --without-qpl \
  --without-xxhash --without-libcurl --without-openssl --without-libxml2 \
  --without-json-c --without-libnl3 --without-uuid --without-selinux \
  --disable-fuse --disable-ublk --disable-s3 --disable-oci --disable-fanotify

# libtool needs -all-static to link libc statically as well.
make -s -j"$(nproc)" LDFLAGS="-all-static" >/dev/null

bin="mkfs/mkfs.erofs"

if ldd "$bin" >/dev/null 2>&1; then
  echo "$bin is not statically linked" >&2
  ldd "$bin" >&2
  exit 1
fi

reported="$("$bin" -V | head -n1)"
if [[ "$reported" != *"$version"* ]]; then
  echo "built binary reports '${reported}', expected version ${version}" >&2
  exit 1
fi

strip "$bin"
install -m 0755 "$bin" "$out_dir/$asset"

cd "$out_dir"
sha256sum "$asset" > "$asset.sha256"

echo "built $out_dir/$asset ($reported)"
cat "$asset.sha256"
