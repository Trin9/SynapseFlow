# SynapseFlow Layered Architecture Boundary Audit (2026-05)

## Scope

This audit verifies the current backend layering boundary after the incremental refactor to:

- `transport`: `internal/api`
- `application`: `internal/application/*`
- `domain`: `internal/domain/*`
- `infrastructure`: `internal/store/*` + external adapters

## Result Summary

- **Business pathways: closed** (`api` handlers delegate to application services).
- **No remaining direct `api -> store` coupling on migrated business use-cases.**
- **A small set of infra-touching locations remains by design** (initialization/probe/middleware side effect).
- **API transport split has advanced to responsibility-based files** and no longer concentrates all handlers in one `server.go`.

## Closed Boundaries (Completed)

- API handler decomposition in `internal/api`:
  - `server_dag.go` (DAG CRUD + run)
  - `server_execution.go` (inline run/resume/get/list/nodes)
  - `server_workspace.go` (episodes/workspace views)
  - `server_system_auth.go` (health/live/metrics/token)
  - `webhook.go` (alert webhook)
  - `server_router.go` (route registration)
  - `server_bootstrap.go` (server wiring/bootstrap)
  - `server_helpers.go` (shared helper functions)
- DAG use-cases routed through `application/dag.Service`
  - create/list/get/update/delete
  - DAG resolution for run and webhook matching
- Execution use-cases routed through `application/execution.Service`
  - run/resume
  - get/list/nodes query handlers
- Workspace use-cases routed through `application/workspace.Service`
  - summary/trigger/review/replay/dossier/memory/comparison
  - episodes list/get
- Ops read use-cases routed through `application/ops.Service`
  - audit list
  - experiences list

## Allowed Exceptions (By Design)

These are not business orchestration paths and are allowed to touch infrastructure directly:

- `NewServer` wiring and infra bootstrap (`db/store/memory` construction + injection)
- `handleHealth` dependency probes (`DB ping`, `MCP ListTools`)
- `auditMiddleware` audit write side effect

## Remaining Clarity Work (Non-Blocking)

- Keep shrinking `server.go` toward a thin shell (`Server` struct + cross-cutting runtime helpers only).
- Decide whether `handleListTools` should stay in `server.go` or move to a dedicated `server_tools.go`.
- If stricter layering is required, move middleware write side-effects and health probing into `application/*` services (see optional hardening below).

## Dependency Rule Status

- Satisfied:
  - `api -> application -> domain`
  - `application -> infrastructure`
- Satisfied constraints:
  - no `domain -> api`
  - no `store -> api dto`
  - no direct `api -> store` for migrated business flows

## Optional Hardening (Not Required for Current DoD)

If stricter purity is required later:

- Introduce `application/system.Service` for health probe orchestration
- Introduce `application/audit.Service` for middleware audit writes

Current state is considered correct and acceptable for layered architecture goals.
