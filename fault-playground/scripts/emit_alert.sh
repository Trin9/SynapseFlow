#!/usr/bin/env bash

set -euo pipefail

ALERT_NAME=${1:-""}
SERVICE=${2:-""}

if [[ -z "${ALERT_NAME}" || -z "${SERVICE}" ]]; then
  echo "usage: emit_alert.sh <alert_name> <service>" >&2
  exit 1
fi

BACKEND_URL=${SYNAPSE_BACKEND_URL:-"http://localhost:8081"}

echo "Emitting alert: ${ALERT_NAME} for service: ${SERVICE} ..."

curl -s -X POST "${BACKEND_URL}/api/v1/webhook/alert" \
  -H "Content-Type: application/json" \
  -d "{
    \"commonLabels\": {
      \"alertname\": \"${ALERT_NAME}\",
      \"service\": \"${SERVICE}\"
    },
    \"alerts\": [
      {
        \"labels\": {
          \"auto_analyse\": \"true\"
        },
        \"status\": \"firing\"
      }
    ]
  }"

echo -e "\nAlert emitted."
