#!/usr/bin/env bash

set -euo pipefail

ALERT_NAME=${1:-""}

if [[ -z "${ALERT_NAME}" ]]; then
  echo "usage: emit_alert.sh <alert_name>" >&2
  exit 1
fi

echo "emit alert: ${ALERT_NAME} (not implemented)"
