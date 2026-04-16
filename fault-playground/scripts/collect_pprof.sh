#!/usr/bin/env bash

set -euo pipefail

SERVICE=${1:-""}
SECONDS=${2:-"10"}

if [[ -z "${SERVICE}" ]]; then
  echo "usage: collect_pprof.sh <service> [seconds]" >&2
  exit 1
fi

case ${SERVICE} in
  gateway) PORT=8080 ;;
  svc-a)   PORT=8081 ;;
  svc-b)   PORT=8082 ;;
  svc-c)   PORT=8083 ;;
  svc-d)   PORT=8084 ;;
  *)
    echo "Unknown service: ${SERVICE}" >&2
    exit 1
    ;;
esac

echo "Collecting pprof (CPU profile) for ${SERVICE} on port ${PORT} for ${SECONDS}s ..."

OUTPUT_FILE="pprof_${SERVICE}_$(date +%Y%m%d_%H%M%S).pb.gz"

curl -s -o "${OUTPUT_FILE}" "http://localhost:${PORT}/debug/pprof/profile?seconds=${SECONDS}"

echo "Profile saved to ${OUTPUT_FILE}"
