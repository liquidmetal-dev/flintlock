#!/bin/bash -ex

UPSTREAM_ORG=firecracker-microvm
FORK_ORG=weaveworks

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

read -r bump latest < <(should_bump)
if [[ "$bump" == "true" ]]; then
    echo "$latest"
fi
