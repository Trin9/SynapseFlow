#!/usr/bin/env bash

set -euo pipefail

profile="boutique"
backend_url="${SYNAPSE_BASE_URL:-http://127.0.0.1:8080}"
poll_limit="${POLL_LIMIT:-90}"
poll_interval="${POLL_INTERVAL_SECONDS:-2}"
output_json="false"
report_file="${REPORT_FILE:-}"

# Optional auth for protected environments.
api_key="${SYNAPSE_API_KEY:-}"
bearer_token="${SYNAPSE_BEARER_TOKEN:-}"

workflow_file=""
expected_episode_type=""
expected_handles=""
require_verdict=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    boutique|fault-lab|custom)
      profile="$1"
      ;;
    --json)
      output_json="true"
      ;;
    --report-file)
      shift
      if [[ $# -eq 0 ]]; then
        echo "--report-file requires a path" >&2
        exit 1
      fi
      report_file="$1"
      ;;
    -h|--help)
      echo "usage: $0 [boutique|fault-lab|custom] [--json] [--report-file <path>]"
      echo "env: REPORT_FILE=auto writes to SynapseDoc/verification/reports/<timestamp>.json"
      exit 0
      ;;
    *)
      echo "unsupported argument: $1" >&2
      echo "usage: $0 [boutique|fault-lab|custom] [--json] [--report-file <path>]" >&2
      exit 1
      ;;
  esac
  shift
done

if [[ "$report_file" == "auto" ]]; then
  ts="$(date -u +%Y%m%dT%H%M%SZ)"
  report_file="/home/trin/project/Synapse/SynapseDoc/verification/reports/episode_${profile}_${ts}.json"
fi

case "$profile" in
  boutique)
    workflow_file="${WORKFLOW_FILE:-/home/trin/project/Synapse/SynapseFlow/backend/cmd/synapse/boutique_checkout_consistency_audit.json}"
    expected_episode_type="action_verification"
    expected_handles="session_id,product_id"
    require_verdict="${REQUIRE_VERDICT:-true}"
    ;;
  fault-lab)
    workflow_file="${WORKFLOW_FILE:-/home/trin/project/Synapse/SynapseFlow/backend/cmd/synapse/checkout_payment_unreachable_agent_loop.json}"
    expected_episode_type="investigation_step"
    expected_handles="${REQUIRED_HANDLES:-}"
    require_verdict="${REQUIRE_VERDICT:-false}"
    ;;
  custom)
    workflow_file="${WORKFLOW_FILE:-}"
    expected_episode_type="${EXPECTED_EPISODE_TYPE:-}"
    expected_handles="${REQUIRED_HANDLES:-}"
    require_verdict="${REQUIRE_VERDICT:-true}"
    ;;
  *)
    echo "unsupported profile: $profile" >&2
    echo "usage: $0 [boutique|fault-lab|custom]" >&2
    exit 1
    ;;
esac

if [[ -z "$workflow_file" || ! -f "$workflow_file" ]]; then
  echo "workflow file not found: $workflow_file" >&2
  exit 1
fi

api_headers=()
if [[ -n "$api_key" ]]; then
  api_headers+=("-H" "X-API-Key: $api_key")
fi
if [[ -n "$bearer_token" ]]; then
  api_headers+=("-H" "Authorization: Bearer $bearer_token")
fi

echo "[1/5] submit workflow profile=$profile"
run_response="$(curl -sS -X POST -H "Content-Type: application/json" "${api_headers[@]}" "$backend_url/api/v1/run" --data-binary @"$workflow_file")"

execution_id="$(RUN_RESPONSE="$run_response" python3 - <<'PY'
import json
import os
import sys

raw = os.environ.get("RUN_RESPONSE", "")
try:
    payload = json.loads(raw)
except json.JSONDecodeError:
    print("")
    sys.exit(0)

eid = payload.get("execution_id", "")
print(eid)
PY
)"

if [[ -z "$execution_id" ]]; then
  echo "failed to parse execution_id from response:" >&2
  echo "$run_response" >&2
  exit 1
fi

echo "execution_id=$execution_id"

echo "[2/5] poll execution status"
execution_status=""
for ((attempt = 1; attempt <= poll_limit; attempt++)); do
  exec_response="$(curl -sS "${api_headers[@]}" "$backend_url/api/v1/executions/$execution_id")"
  execution_status="$(EXEC_RESPONSE="$exec_response" python3 - <<'PY'
import json
import os

raw = os.environ.get("EXEC_RESPONSE", "")
try:
    payload = json.loads(raw)
except json.JSONDecodeError:
    print("")
    raise SystemExit(0)
print(payload.get("status", ""))
PY
)"

case "$execution_status" in
    completed|failed|suspended)
      echo "execution reached terminal status: $execution_status"
      break
      ;;
    *)
      echo "poll $attempt/$poll_limit status=${execution_status:-unknown}"
      sleep "$poll_interval"
      ;;
  esac
done

if [[ "$execution_status" != "completed" && "$execution_status" != "failed" && "$execution_status" != "suspended" ]]; then
  echo "timed out waiting for execution terminal status" >&2
  exit 1
fi

echo "[3/5] fetch episodes"
episodes_response="$(curl -sS "${api_headers[@]}" "$backend_url/api/v1/executions/$execution_id/episodes")"

echo "[4/5] evaluate episode assertions"
report_json="$(EPISODES_RESPONSE="$episodes_response" EXPECTED_EPISODE_TYPE="$expected_episode_type" REQUIRED_HANDLES="$expected_handles" REQUIRE_VERDICT="$require_verdict" EXECUTION_STATUS="$execution_status" python3 - <<'PY'
import json
import os

raw = os.environ.get("EPISODES_RESPONSE", "")
expected_type = os.environ.get("EXPECTED_EPISODE_TYPE", "").strip()
required_handles = [x.strip() for x in os.environ.get("REQUIRED_HANDLES", "").split(",") if x.strip()]
require_verdict = os.environ.get("REQUIRE_VERDICT", "true").lower() == "true"
execution_status = os.environ.get("EXECUTION_STATUS", "")

result = {
    "execution_status": execution_status,
    "episode_count": 0,
    "episode_id": "",
    "episode_type": "",
    "episode_status": "",
    "evidence_count": 0,
    "collector_spec_count": 0,
    "collector_types": [],
    "handles": [],
    "verdict_present": False,
    "errors": [],
}

try:
    payload = json.loads(raw)
except json.JSONDecodeError:
    result["errors"].append("episodes_response_invalid_json")
    print(json.dumps(result))
    raise SystemExit(0)

episodes = payload.get("episodes", [])
if not isinstance(episodes, list):
    result["errors"].append("episodes_not_list")
    print(json.dumps(result))
    raise SystemExit(0)

result["episode_count"] = len(episodes)
if not episodes:
    result["errors"].append("episode_missing")
    print(json.dumps(result))
    raise SystemExit(0)

ep = episodes[0]
result["episode_id"] = ep.get("id", "")
result["episode_type"] = ep.get("episode_type", "")
result["episode_status"] = ep.get("status", "")

evidence = ep.get("evidence", [])
if isinstance(evidence, list):
    result["evidence_count"] = len(evidence)
    collector_types = set()
    spec_count = 0
    for item in evidence:
        if isinstance(item, dict) and item.get("collector_spec") is not None:
            spec_count += 1
            spec = item.get("collector_spec")
            if isinstance(spec, dict):
                ctype = spec.get("collector_type")
                if ctype:
                    collector_types.add(str(ctype))
    result["collector_spec_count"] = spec_count
    result["collector_types"] = sorted(collector_types)

handles = ep.get("handles", [])
handle_types = []
if isinstance(handles, list):
    for h in handles:
        if isinstance(h, dict) and h.get("type"):
            handle_types.append(str(h.get("type")))
result["handles"] = sorted(handle_types)

verdict = ep.get("verdict")
result["verdict_present"] = verdict is not None

if expected_type and result["episode_type"] != expected_type:
    result["errors"].append(f"episode_type_mismatch:{result['episode_type']}!={expected_type}")

if result["episode_status"] not in {"converged", "failed", "escalated"}:
    result["errors"].append("episode_status_not_terminal")

if result["evidence_count"] == 0:
    result["errors"].append("evidence_missing")

if result["collector_spec_count"] == 0:
    result["errors"].append("collector_spec_missing")

for req in required_handles:
    if req not in handle_types:
        result["errors"].append(f"handle_missing:{req}")

if require_verdict and not result["verdict_present"]:
    result["errors"].append("verdict_missing")

print(json.dumps(result))
PY
)"

echo "[5/5] report"
if [[ -n "$report_file" ]]; then
  mkdir -p "$(dirname "$report_file")"
  printf '%s\n' "$report_json" > "$report_file"
fi

REPORT_JSON="$report_json" OUTPUT_JSON="$output_json" REPORT_FILE="$report_file" python3 - <<'PY'
import json
import os

data = json.loads(os.environ.get("REPORT_JSON", "{}"))
output_json = os.environ.get("OUTPUT_JSON", "false").lower() == "true"
report_file = os.environ.get("REPORT_FILE", "")

if output_json:
    print(json.dumps(data))
    if report_file:
        print(f"report_file={report_file}")
else:
    print(f"execution_status={data.get('execution_status')}")
    print(f"episode_count={data.get('episode_count')}")
    print(f"episode_id={data.get('episode_id')}")
    print(f"episode_type={data.get('episode_type')}")
    print(f"episode_status={data.get('episode_status')}")
    print(f"evidence_count={data.get('evidence_count')}")
    print(f"collector_spec_count={data.get('collector_spec_count')}")
    print(f"collector_types={','.join(data.get('collector_types', []))}")
    print(f"handles={','.join(data.get('handles', []))}")
    print(f"verdict_present={data.get('verdict_present')}")
    if report_file:
        print(f"report_file={report_file}")

errors = data.get("errors", [])
if errors:
    if output_json:
        pass
    else:
        print("result=FAIL")
        print("errors=")
        for e in errors:
            print(f"- {e}")
    raise SystemExit(1)

if output_json:
    pass
else:
    print("result=PASS")
PY
