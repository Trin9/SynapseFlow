# SynapseFlow Layered Migration Plan (2026-05)

## Purpose

Track as-built migration status, remaining work, and execution order so future batches do not lose context.

## Tracked Roadmap

### 1) Low risk, high return: split `internal/api/server.go`

Status: **Mostly complete**

Completed slices:

- Handler files: `server_dag.go`, `server_execution.go`, `server_workspace.go`, `server_system_auth.go`, `server_tools.go`, `webhook.go`
- Composition files: `server_router.go`, `server_bootstrap.go`, `server_helpers.go`

Remaining:

- Keep `server.go` thin and avoid re-accumulating responsibilities.

### 2) Medium risk, medium return: system-level exception sinking

Status: **Completed**

Completed in current stage:

- Introduced `internal/application/system/service.go`.
- Moved health dependency probing into `application/system.Service`.
- API `handleHealth` now consumes `system.Service` probe result.

Completed in this batch:

- Introduced `internal/application/audit/service.go`.
- API `auditMiddleware` now delegates write orchestration to `application/audit.Service`.

Remaining:

- Keep monitoring for any direct transport-side write orchestration reintroduction.

### 3) Medium-high risk, high return: move domain-heavy types out of `pkg/models`

Status: **Partially started**

Completed seeds:

- `internal/domain/episode/*` and `internal/domain/execution/*` transitions/typing foundations.
- First migration slice added: `internal/domain/episode/enums.go` with `EpisodeType` domain enum and model mapping.
- Execution auto-create path now validates `metadata.episode_type` via domain enum before creating Episode records.
- Second migration slice added: domain `EpisodeStatus` enum with model mapping; write paths now consume domain status mapping for pending/in_progress/converged transitions.
- Follow-up hardening: domain `EpisodeStatus` mapping now also used for pending checks in episode writer and transition policy mapping in `internal/domain/episode/transitions.go`.
- Projection alignment: workspace projector replay-percentage status buckets now use domain `EpisodeStatus` mappings instead of direct model constants.
- Projection follow-up: process-trace action-stage detection now uses domain `EpisodeType` mapping (`action_verification`) instead of direct model enum.

Remaining:

- Incrementally migrate enum semantics and domain-heavy structs while preserving store/json compatibility.

### 4) Continuous evolution: string-enum semantics hardening

Status: **In progress**

Completed examples:

- Execution status boundary validation.
- Review status domain typing and validation.
- Resume action validation and typed application input.
- Added shared enum validators in `pkg/models` for Episode and human-action enums.
- Execution resume flow now uses enum helper checks (`IsResumeAction`) and typed node-result status comparison in application layer.

Remaining:

- Continue endpoint-by-endpoint boundary validation for string-semantic fields.

## Batch Verification Gate

Per batch:

1. LSP diagnostics clean on changed files.
2. `go test ./internal/api` passes.
3. Manual API QA for touched endpoints.
4. Commit and push.

Known pre-existing repo-wide issue:

- `go test ./...` fails in `internal/api/docs` due to missing `github.com/swaggo/swag` module.

Latest batch verification (EpisodeStatus migration follow-up):

- Automated: `go test ./internal/domain/episode ./internal/application/execution ./internal/engine ./internal/projection/workspace ./internal/api`
- Automated regression: `go test ./internal/api -run TestEpisodeLifecycle_AutoCreateAndConverge -count=10`
- Manual API QA:
  - `GET /health` returns `{"status":"ok",...}` on local `:18080`.
  - `POST /api/v1/run` with `metadata.episode_type=action_verification` returns accepted execution.
  - `GET /api/v1/executions/:id` reaches `status=completed`.
  - `GET /api/v1/executions/:id/episodes` confirms episode `status=converged` with evidence and verdict.
  - `GET /api/v1/executions/:id/episodes?view=summary` confirms `default_replay_percent=100` for converged status.

## Next Recommended Order

1. Continue domain-type extraction from `pkg/models` in small reversible slices.
2. Continue enum hardening at transport boundary.
3. Add focused tests around audit application service behavior (optional hardening).
