#!/usr/bin/env bash

set -euxo pipefail

if [[ $(id -u) != 0 ]]; then
  echo "Run this script as root..." >&2
  exit 1
fi

GO_VERSION="1.17.2"
INSTALL_ROOT="/usr/local"

# install packages
apt update
apt install -y \
    jq \
    wget \
    unzip \
    curl \
    tmux \
    gcc \
    vim \
    iproute2 \
    bc \
    dmsetup \
    make \
    iproute2 \
    git

# install go
export PATH="$PATH:$INSTALL_ROOT/go/bin"
curl -sL "https://golang.org/dl/go$GO_VERSION.linux-amd64.tar.gz" | tar xz -C "$INSTALL_ROOT" && \
  go version

# install firecracker
wget -O /tmp/firecracker_macvtap.zip https://github.com/weaveworks/flintlock/files/7278467/firecracker_macvtap.zip && \
    unzip -o /tmp/firecracker_macvtap.zip -d "$INSTALL_ROOT/bin" && \
    firecracker --version

# install and setup containerd
curl -sL https://api.github.com/repos/containerd/containerd/releases/latest 2>/dev/null | \
    jq -r '.assets[] | select(.browser_download_url | test("containerd-\\d.\\d.\\d-linux-amd64.tar.gz$")) | .browser_download_url' | \
    xargs curl -sL | tar xz -C "$INSTALL_ROOT" && containerd --version && ctr --version

# install grpcurl
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest && \
  grpcurl --version
