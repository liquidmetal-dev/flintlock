#!/bin/bash -ex

UPSTREAM_ORG=firecracker-microvm
FORK_ORG=weaveworks
BOOTSTRAP_SH="hack/scripts/bootstrap.sh"

latest_release() {
    curl -s "https://api.github.com/repos/$1/firecracker/releases/latest" | awk -F'"' '/tag_name/ {printf $4}'
}

should_bump() {
    latest=$(latest_release "$UPSTREAM_ORG")
    current=$(latest_release "$FORK_ORG")
    result="false"
    if [[ "$current" != *"$latest"* ]]; then
        result="true"
    fi
    echo "$result" "$latest"
}

bump_version() {
    sed -i -e "s|\(FIRECRACKER_VERSION=\).*|\1\"$1\"|" "$BOOTSTRAP_SH"
}

read -r bump latest < <(should_bump)
if [[ "$bump" == "true" ]]; then
    bump_version "$latest"
    echo "$latest"
fi
