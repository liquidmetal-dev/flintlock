#!/bin/bash

mv /usr/local/go/bin/go /usr/local/bin

mkdir -p /root/work && cd /root/work
# TODO make it so that repo/branch etc can be configured for local runs / copy up binary / clone at commit etc
git clone https://github.com/weaveworks/flintlock --depth 1

touch /flintlock_ready
