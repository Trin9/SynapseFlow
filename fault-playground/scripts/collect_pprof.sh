#!/usr/bin/env bash

set -euo pipefail

SERVICE=${1:-""}

if [[ -z "${SERVICE}" ]]; then
  echo "usage: collect_pprof.sh <service>" >&2
  exit 1
fi

echo "collect pprof for ${SERVICE} (not implemented)"
