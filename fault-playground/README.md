# Synapse Fault Playground

This directory hosts a scaffold for the Synapse Fault Playground described in
`docs/verification/SYNAPSE_FAULT_LAB_SPEC.md`. It provides a minimal service
layout, scenario metadata, and stubbed scripts so the real implementations can
be filled in later.

## Structure

- `cmd/`: service entrypoints (gateway, svc-a/b/c/d)
- `internal/chain/`: request chain orchestration
- `internal/faults/`: fault injection hooks
- `internal/observability/`: logging, metrics, pprof stubs
- `scenarios/`: scenario metadata YAML
- `scripts/`: helper scripts to trigger scenarios and collect evidence
- `deployments/`: runtime manifests (docker-compose)
- `docs/`: scenario catalog

## Phase 1 focus

The primary target is `distributed_nil_pointer`. Implementations are intentionally
left empty for now and should be filled in during feature work.
