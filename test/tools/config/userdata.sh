#!/bin/bash

set -exo pipefail

if [[ $(id -u) != 0 ]]; then
	echo "Run this script as root..." >&2
	exit 1
fi

GO_VERSION="1.18.5"
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
export GOPATH=/root/go
export PATH="$PATH:$INSTALL_ROOT/go/bin:$GOPATH/bin"
curl -sL "https://golang.org/dl/go$GO_VERSION.linux-amd64.tar.gz" | tar xz -C "$INSTALL_ROOT" &&
	go version

mv /usr/local/go/bin/go /usr/local/bin

mkdir -p /root/work && cd /root/work

if [[ -z $FL_USER ]]; then
	FL_USER=weaveworks
fi

if [[ -z "$FL_BRANCH" ]]; then
	FL_BRANCH=main
fi

git clone "https://github.com/$FL_USER/flintlock" --depth 1 --branch "$FL_BRANCH"

if [[ -z "$SKIP_DIRECT_LVM" ]]; then
	./flintlock/hack/scripts/provision.sh direct_lvm -y
fi

# install latest firecracker
./flintlock/hack/scripts/provision.sh firecracker --version v0.25.2-macvtap

# install and setup containerd
# curl -sL https://api.github.com/repos/containerd/containerd/releases/latest 2>/dev/null |
# 	jq -r '.assets[] | select(.browser_download_url | test("containerd-\\d.\\d.\\d-linux-amd64.tar.gz$")) | .browser_download_url' |
# 	xargs curl -sL | tar xz -C "$INSTALL_ROOT" && containerd --version && ctr --version

# Pinning to container 1.6.6 for a moment while we deal with a change which came
# in with 1.6.7
curl -sL https://github.com/containerd/containerd/releases/download/v1.6.6/containerd-1.6.6-linux-amd64.tar.gz | tar xz -C "$INSTALL_ROOT" && containerd --version && ctr --version

# install grpcurl
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest && \
	grpcurl --version

# install hammertime
go install github.com/warehouse-13/hammertime/releases@latest && \
	hammertime

touch /flintlock_ready
