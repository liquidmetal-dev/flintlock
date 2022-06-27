#!/usr/bin/env bash

host=localhost
port=9090
service="microvm.services.api.v1alpha1.MicroVM"
method="ListMicroVMs"
namespace="ns1"
uid=""

if ! which grpcurl >/dev/null 2>/dev/null; then
  echo "!!! grpcurl is not installed. Please install this awesome tool." >&2
  echo "" >&2
  echo "  go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest" >&2

  exit 1
fi

function help() {
  cat <<EOH
${0} [flags] -s service -m method>
  -H/--host HOST         hostname; default: ${host}
  -p/--port PORT         port; default: ${port}
  -s/--service service   name of the service; default: ${service}
  -m/--method method     name of the method on the service; default: ${method}
  --uid uid              uuid of the microvm
  --namespace namespace  namespace to query; default: ${namespace}
  -h/--help              help

Special:
 * DeleteMicroVM uses --uid
 * ListMicroVMs uses --namespace

Examples:

  ${0} -m ListMicroVMs --namespace ns1
  ${0} -m DeleteMicroVM --uid "microvmuid"
EOH

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
    --namespace)
      namespace=${2}
      shift; shift;
      ;;
    --uid)
      uid=${2}
      shift; shift;
      ;;
    *)
      echo "Unknown flag: ${1}" >&2
      help

      exit 1
  esac
done

pushd "$(dirname "${0}")" > /dev/null

case "${method}" in
  "DeleteMicroVM")
    grpcurl -d "{\"uid\": \"${uid}\"}" \
      -plaintext "${host}:${port}" \
      "${service}/${method}"
    ;;
  "ListMicroVMs")
    grpcurl -d "{\"namespace\": \"${namespace}\"}" \
      -plaintext "${host}:${port}" \
      "${service}/${method}"
    ;;
  *)
    grpcurl -d @ \
      -plaintext "${host}:${port}" \
      "${service}/${method}" \
      < "payload/${method}.json"
    ;;
esac


popd > /dev/null
