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
- Audit write orchestration now routed through `application/audit.Service`.

## Allowed Exceptions (By Design)

These are currently allowed while keeping business orchestration clean:

- `NewServer` wiring and infra bootstrap (`db/store/memory` construction + injection)
- `auditMiddleware` remains transport trigger point but delegates write to `application/audit.Service`

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

## Latest Progress Notes (2026-05 follow-up)

- Runtime-layer Episode enum usage has been aligned to domain mapping in migrated paths (`engine`, `application`, `projection`, `domain transitions`).
- Test-layer Episode enum usage is now also aligned to domain mapping in `internal/engine`, `internal/store`, and `internal/api` tests.
- New writer consistency coverage added for review-state updates: explicit episode targeting and fallback latest-episode selection.
- Domain verdict enum mapping (`EpisodeResult`/`EpisodeConfidence`) is now used in runtime verdict parse and memory-extraction decision paths.
- Domain trigger/evidence enum mapping (`EpisodeTriggerType`/`EpisodeEvidenceType`) is now used in runtime episode creation and evidence-write paths.
- Domain handle enum mapping (`EpisodeHandleType`) is now used in runtime handle-write path and aligned tests.
- Domain human-action enum mapping (`HumanInterventionAction`) is now used for resume default action semantics.
- API transport boundary validation strengthened for pagination (`limit`/`offset`), replay range (`percent`), and resume JSON payload parsing.

## Remaining Gaps (Current)

- Continue extracting additional domain-heavy semantics from `pkg/models` while preserving storage/json compatibility.
- Resolve repo-wide `go test ./...` blocker in `internal/api/docs` (missing `github.com/swaggo/swag`) to restore full-suite green baseline.
