#!/bin/bash -ex

mv /usr/local/go/bin/go /usr/local/bin

mkdir -p /root/work && cd /root/work

if [[ -z $FL_USER ]];then
    FL_USER=weaveworks
fi

if [[ -z "$FL_BRANCH" ]]; then
    FL_BRANCH=main
fi

git clone "https://github.com/$FL_USER/flintlock" --depth 1 --branch "$FL_BRANCH"

touch /flintlock_ready
