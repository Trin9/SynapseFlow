#!/usr/bin/env bash

set -euo pipefail

backend_url="${SYNAPSE_BASE_URL:-http://127.0.0.1:18091}"
workflow_file="${1:-/home/trin/project/Synapse/SynapseFlow/backend/cmd/synapse/checkout_payment_unreachable_agent_loop.json}"
expected_marker="${EXPECTED_REPORT_MARKER:-badAddress:50051}"
poll_limit="${POLL_LIMIT:-60}"

if [[ ! -f "$workflow_file" ]]; then
  echo "workflow file not found: $workflow_file" >&2
  exit 1
fi

echo "Submitting workflow: $workflow_file"
run_response="$(curl -sS -X POST "$backend_url/api/v1/run" -H 'Content-Type: application/json' --data-binary @"$workflow_file")"
execution_id="$(printf '%s' "$run_response" | tr -d '\n' | sed -E 's/.*"execution_id"[[:space:]]*:[[:space:]]*"?([0-9]+)"?.*/\1/')"

if [[ -z "$execution_id" || "$execution_id" == "$run_response" ]]; then
  echo "failed to parse execution_id from response: $run_response" >&2
  exit 1
fi

echo "execution_id=$execution_id"

for ((attempt=1; attempt<=poll_limit; attempt++)); do
  nodes_response="$(curl -sS "$backend_url/api/v1/executions/$execution_id/nodes")"

  if printf '%s' "$nodes_response" | grep -q '"node_id":"final_report"'; then
    if printf '%s' "$nodes_response" | grep -q '"node_id":"final_report".*"status":"success"'; then
      if printf '%s' "$nodes_response" | grep -q "$expected_marker"; then
        echo "regression passed: final_report contains $expected_marker"
        exit 0
      fi
      echo "final_report succeeded, but expected marker not found: $expected_marker" >&2
      printf '%s\n' "$nodes_response" >&2
      exit 1
    fi
  fi

  if printf '%s' "$nodes_response" | grep -q '"status":"error"'; then
    echo "execution failed before final_report succeeded" >&2
    printf '%s\n' "$nodes_response" >&2
    exit 1
  fi

  echo "poll $attempt/$poll_limit: execution still running"
  sleep 2
done

echo "timed out waiting for final_report" >&2
exit 1