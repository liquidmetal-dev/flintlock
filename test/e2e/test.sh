#!/bin/bash

go test -timeout 30m -p 1 -v -tags=e2e "$(dirname "$(realpath "$0")")"/... "$@"
