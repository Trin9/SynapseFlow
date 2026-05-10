# SynapseFlow Layered Architecture Boundary Audit (2026-05)

## Scope

This audit verifies backend layering boundaries after incremental refactor:

- `transport`: `internal/api`
- `application`: `internal/application/*`
- `domain`: `internal/domain/*`
- `infrastructure`: `internal/store/*` and external adapters

## Result Summary

- Business pathways are closed through application services.
- No direct `api -> store` coupling remains in migrated business use-cases.
- A small set of infra-touching points remains by design.
- API transport has been decomposed by responsibility and is no longer concentrated in one file.

## Closed Boundaries (Completed)

- API decomposition in `internal/api`:
  - `server_dag.go`
  - `server_execution.go`
  - `server_workspace.go`
  - `server_system_auth.go`
  - `server_tools.go`
  - `webhook.go`
  - `server_router.go`
  - `server_bootstrap.go`
  - `server_helpers.go`
- DAG use-cases routed through `application/dag.Service`.
- Execution use-cases routed through `application/execution.Service`.
- Workspace use-cases routed through `application/workspace.Service`.
- Ops reads routed through `application/ops.Service`.
- System health probing now routed through `application/system.Service`.

## Allowed Exceptions (By Design)

These are currently allowed while keeping business orchestration clean:

- `NewServer` wiring and infra bootstrap (`db/store/memory` construction + injection)
- `auditMiddleware` audit write side effect

## Dependency Rule Status

- Satisfied:
  - `api -> application -> domain`
  - `application -> infrastructure`
- Satisfied constraints:
  - no `domain -> api`
  - no `store -> api dto`
  - no direct `api -> store` for migrated business flows

## Remaining Clarity Work (Non-Blocking)

- Keep `server.go` as thin shell only.
- Consider moving audit write side-effect into dedicated application service.
- Continue enum boundary validation and internal strong typing migration.
