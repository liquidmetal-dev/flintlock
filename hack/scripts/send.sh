#!/usr/bin/env bash

host=localhost
port=9090
service="microvm.services.api.v1alpha1.MicroVM"
method="ListMicroVMs"

if ! which grpcurl >/dev/null 2>/dev/null; then
  echo "!!! grpcurl is not installed. Please install this awesome tool." >&2
  echo "" >&2
  echo "  go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest" >&2

  exit 1
fi

function help() {
  echo "${0} [-h HOST] [-p PORT] -s service -m method>"
  echo "  -H/--host HOST         hostname; default: ${host}"
  echo "  -p/--port PORT         port; default: ${port}"
  echo "  -s/--service service   name of the service; default: ${service}"
  echo "  -m/--method method     name of the method on the service; default: ${method}"
  echo "  -h/--help              help"

  exit 0
}

while [[ "$#" -gt 0 ]]; do
  case "${1}" in
    -H|--host)
      host=${2}
      shift; shift
      ;;
    -p|--port)
      port=${2}
      shift; shift
      ;;
    -s|--service)
      service=${2}
      shift; shift
      ;;
    -m|--method)
      method=${2}
      shift; shift
      ;;
    -h|--help)
      help

      exit 0
      ;;
    *)
      echo "Unknown flag: ${1}" >&2
      help

      exit 1
  esac
done

pushd "$(dirname "${0}")" > /dev/null

grpcurl -d @ \
  -plaintext "${host}:${port}" \
  "${service}/${method}" \
  < "payload/${method}.json"

popd > /dev/null
