#!/usr/bin/env bash

set -euo pipefail

SCENARIO_NAME=${1:-""}

if [[ -z "${SCENARIO_NAME}" ]]; then
  echo "usage: trigger_scenario.sh <scenario_name>" >&2
  exit 1
fi

echo "trigger scenario: ${SCENARIO_NAME} (not implemented)"
