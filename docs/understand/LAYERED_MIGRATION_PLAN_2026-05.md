# SynapseFlow Layered Migration Plan (2026-05)

## Purpose

This document records the **as-built migration progress**, remaining steps, and execution order for the backend layered refactor.

Primary goals:

- Preserve behavior while clarifying architecture.
- Enforce `api -> application -> domain` for business pathways.
- Keep infra concerns explicit and bounded.
- Avoid knowledge loss for subsequent migration batches.

## Scope

Backend directories in scope:

- `backend/internal/api`
- `backend/internal/application/*`
- `backend/internal/domain/*`
- `backend/pkg/models`

## Migration Roadmap (Tracked)

### 1) Low Risk, High Return: Split `internal/api/server.go` by responsibility

Status: **In Progress (majority completed)**

Completed:

- `server_execution.go`
  - run inline / run workflow / start execution
  - get execution / list executions / execution nodes
  - resume execution
- `server_workspace.go`
  - episodes list/get
  - execution workspace read endpoints (summary/trigger/review/replay/dossier/memory/comparison)
  - view builders for workspace responses
- `server_system_auth.go`
  - `handleHealth`, `handleLive`, `handleIssueToken`, `handleMetrics`
- `server_dag.go`
  - DAG create/list/get/update/delete
  - run DAG
- `webhook.go`
  - webhook-specific payload parsing and DAG matching flow
- `server_router.go`
  - route registration and API grouping
- `server_bootstrap.go`
  - `NewServer` wiring, infra bootstrapping, scheduler + services construction
- `server_helpers.go`
  - common helpers (`writeError`, DAG validation helpers, API key parsing, execution notification helpers)

Remaining:

- Keep `server.go` as a thin shell and move any new cross-cutting additions to dedicated files.
- Optional: extract `handleListTools` into `server_tools.go` if tool endpoints grow.

### 2) Medium Risk, Medium Return: Move system-level exceptions into `application/system`

Status: **Planned (not started)**

Current allowed exceptions:

- health dependency probing (`DB ping`, `MCP ListTools`)
- audit middleware write side-effect

Planned target:

- `application/system.Service` for health probing aggregation
- `application/audit.Service` for middleware write orchestration

Rationale:

- Further tighten transport purity and centralize system policies.

### 3) Medium-High Risk, High Return: Migrate domain-heavy types out of `pkg/models`

Status: **Partially started**

Completed seeds:

- `internal/domain/episode/*` (review semantics + transitions)
- `internal/domain/execution/*` (terminal status resolution)

Remaining large area:

- `pkg/models` still contains many domain-heavy Episode and human-intervention structures.

Incremental strategy:

1. Extract enum semantics and validators first.
2. Extract transition logic and mapping functions.
3. Migrate structs with compatibility boundaries (store/json contracts) preserved.

### 4) Continuous Hardening: String-enum boundary validation + internal strong typing

Status: **In Progress**

Completed:

- `ExecutionStatus.IsValid()` introduced and enforced at API boundary.
- Review status moved to domain enum (`internal/domain/episode`) with validation.
- Resume action validated at API boundary and typed in application layer.

Planned next focus:

- Enumerate and validate additional string-semantics fields (for example node result status / action types / episode state-like fields).

## Batch History (High-Level)

Recent completed batches include:

- Route business handlers through application services (execution/workspace/dag/ops).
- Harden enum semantics for review and execution flows.
- Split API handlers and server composition into responsibility-based files.

For exact code history, use git log with the recent `refactor(backend)` commits on `master`.

## Execution Principles For Remaining Batches

- One batch = one coherent responsibility change.
- No behavior drift: validate via tests + live API surface checks.
- Prefer smallest safe change; avoid opportunistic refactors.
- Keep pre-existing unrelated failures out of scope and explicitly report them.

## Verification Gates (Per Batch)

Required per migration batch:

1. `lsp_diagnostics` clean on changed files (errors = 0).
2. `go test ./internal/api` passes.
3. Manual API QA on endpoints touched by the batch.
4. Commit and push with clear message.

Known pre-existing repository issue:

- `go test ./...` fails in `internal/api/docs` due to missing module `github.com/swaggo/swag`.
- This is tracked as unrelated to current layering refactor changes.

## Next Recommended Batch Order

1. (Optional) `server_tools.go` extraction if tools endpoint scope expands.
2. `application/system.Service` introduction (health probe orchestration).
3. `application/audit.Service` introduction (audit write orchestration).
4. Domain-type migration from `pkg/models` in narrow, reversible slices.
5. Continue enum hardening at transport boundary for newly identified string semantics.

## Ownership Notes

- This document should be updated after every migration batch merge.
- Keep sections "Status", "Batch History", and "Next Recommended Batch Order" current.
