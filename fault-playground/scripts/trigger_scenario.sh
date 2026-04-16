#!/usr/bin/env bash

set -euo pipefail

SCENARIO_NAME=${1:-""}

if [[ -z "${SCENARIO_NAME}" ]]; then
  echo "usage: trigger_scenario.sh <scenario_name>" >&2
  exit 1
fi

echo "Triggering scenario: ${SCENARIO_NAME} ..."

SERVICES=("svc-a" "svc-b" "svc-c" "svc-d")

for SVC in "${SERVICES[@]}"; do
  echo "Setting scenario on ${SVC} ..."
  curl -s -X POST "http://localhost:8080/admin/scenario" \
    -H "Host: ${SVC}" \
    -H "Content-Type: application/json" \
    -d "{\"name\": \"${SCENARIO_NAME}\", \"fault\": \"${SCENARIO_NAME}\"}" || echo "Failed to reach ${SVC}"
done

echo "Scenario ${SCENARIO_NAME} triggered."
