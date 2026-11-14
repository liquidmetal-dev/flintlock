#!/usr/bin/env bash
# Build flintlock-overlay-init and pack it into a gzip compressed cpio
# (newc) archive with the binary at /init. The kernel unpacks the archive as
# the initramfs and runs /init.
#
# Usage: build-overlay-initrd.sh <output file> [goarch]
#
# Environment:
#   VERSION, BUILD_DATE, GIT_COMMIT   Set the build information of the binary.
#                                     Each defaults to the value git reports.
set -euo pipefail

output="${1:?usage: $0 <output file> [goarch]}"
goarch="${2:-$(go env GOARCH)}"

repo_root="$(git -C "$(dirname "${BASH_SOURCE[0]}")" rev-parse --show-toplevel)"
version_pkg="github.com/liquidmetal-dev/flintlock/internal/version"

VERSION="${VERSION:-$(git -C "${repo_root}" describe --tags --abbrev=0)}"
BUILD_DATE="${BUILD_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
GIT_COMMIT="${GIT_COMMIT:-$(git -C "${repo_root}" rev-parse --short HEAD)}"

staging="$(mktemp -d)"
trap 'rm -rf "${staging}"' EXIT

CGO_ENABLED=0 GOOS=linux GOARCH="${goarch}" go build \
  -trimpath \
  -ldflags "-s -w -X ${version_pkg}.Version=${VERSION} -X ${version_pkg}.BuildDate=${BUILD_DATE} -X ${version_pkg}.CommitHash=${GIT_COMMIT}" \
  -o "${staging}/init" \
  "${repo_root}/cmd/flintlock-overlay-init"

mkdir -p "$(dirname "${output}")"
case "${output}" in
  /*) output_abs="${output}" ;;
  *) output_abs="${PWD}/${output}" ;;
esac

# A fixed modification time, --reproducible and -R 0:0 make the archive
# depend only on the binary. gzip -n leaves the name and time stamp out of
# the header.
touch -d "@${SOURCE_DATE_EPOCH:-0}" "${staging}/init"
(
  cd "${staging}"
  printf 'init\n' | cpio --quiet -o -H newc --reproducible -R 0:0 | gzip -9 -n
) > "${output_abs}"

echo "wrote ${output} (${goarch})"
