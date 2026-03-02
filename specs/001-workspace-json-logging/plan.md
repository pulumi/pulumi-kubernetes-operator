# Implementation Plan: Workspace Agent Structured JSON Logging

**Branch**: `001-workspace-json-logging` | **Date**: 2026-03-10 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/001-workspace-json-logging/spec.md`

## Summary

Add structured JSON logging support to the workspace agent, configurable via environment variable
(`AGENT_LOG_FORMAT`), Workspace CR field (`spec.logFormat`), and Helm value
(`workspace.logFormat`). Console format remains the default. Extends to optionally log Pulumi
engine events as structured JSON in place of human-readable progress stream output
(`AGENT_PULUMI_JSON_OUTPUT` / `spec.pulumiJsonOutput`).

## Technical Context

**Language/Version**: Go 1.22+ (module: `github.com/pulumi/pulumi-kubernetes-operator/v2`)
**Primary Dependencies**: `go.uber.org/zap` (structured logging), `controller-runtime` (reconciler
framework), Pulumi Automation API SDK v3 (`github.com/pulumi/pulumi/sdk/v3 v3.220.0`),
`kubebuilder` v4.2.0 (CRD generation markers), `go.uber.org/zap/zapio` (writer bridge)
**Storage**: Kubernetes API server (CRD state), StatefulSet pod specs
**Testing**: `envtest` (K8s 1.32.0), `go test`, Helm `ct lint` (chart-testing)
**Target Platform**: Kubernetes operator + in-cluster gRPC agent
**Project Type**: Kubernetes Operator + gRPC Agent (two binaries)
**Performance Goals**: Logger init is a one-time startup cost; no per-request overhead concern.
`zapio.Writer` → logger pipeline for Pulumi output is the existing hot path — no change.
**Constraints**: Console remains default (zero breaking change); fail-fast on invalid env var
values; CR field takes precedence over cluster-wide default; no `--show-secrets` flag allowed.
**Scale/Scope**: One agent process per workspace pod; one controller reconciling N workspaces.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

Verify compliance with the [project constitution](.specify/memory/constitution.md):

| Gate | Principle | Status |
|------|-----------|--------|
| `golangci-lint` passes with `.golangci.yml` | I. Code Quality | ✅ All new Go follows existing style; `gosec`/`goheader` requirements met |
| Apache 2.0 license header on all new files | I. Code Quality | ✅ No new files expected; headers added to any new file |
| Unit tests via `envtest` for new controllers/reconcilers | II. Testing | ✅ WorkspaceReconciler env var propagation tested; agent logger switching unit-tested |
| E2E tests for Stack→Workspace→Update→Agent workflow changes | II. Testing | ✅ Covered via existing e2e suite; smoke test validates format switching |
| Codecov coverage not decreased | II. Testing | ✅ New code paths explicitly covered by new unit tests |
| CRD status conditions use standard types (Ready/Reconciling/Stalled) | III. UX | ✅ No new conditions added; existing pattern unchanged |
| Helm chart lints cleanly (`ct lint`) | III. UX | ✅ New `workspace.logFormat` value follows existing `controller.logFormat` pattern |
| Reconcile loops delegate blocking ops to agent via gRPC | IV. Performance | ✅ No blocking ops added; env var injection is fast, idempotent field append |
| CHANGELOG.md updated with user-facing changes | V. Documentation | ☐ Pending Phase 7 (`T019`) |
| CRD docs regenerated (`make generate-crdocs`) if schema changed | V. Documentation | ☐ Pending Phase 7 (`T020`); required because `spec.logFormat` and `spec.pulumiJsonOutput` are new CRD fields |

**Complexity Justification** (fill only if any gate has a documented exception): None.

## Project Structure

### Documentation (this feature)

```text
specs/001-workspace-json-logging/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

```text
agent/
├── cmd/
│   └── root.go               # Logger init — add AGENT_LOG_FORMAT env var reading
├── pkg/
│   └── server/
│       └── server.go         # Add PulumiJsonOutput option; toggle ProgressStreams/EventStreams

operator/
├── api/
│   └── auto/v1alpha1/
│       └── workspace_types.go  # Add LogFormat + PulumiJsonOutput fields to WorkspaceSpec
├── internal/
│   ├── controller/
│   │   └── auto/
│   │       └── workspace_controller.go  # newStatefulSet() — inject env vars from CR fields
│   └── webhook/
│       └── auto/v1alpha1/
│           └── workspace_webhook.go     # No change needed (CRD enum validation handles it)

deploy/
├── crds/                     # Regenerated: make generate-crds
│   └── auto.pulumi.com_workspaces.yaml
├── helm/pulumi-operator/
│   ├── values.yaml           # Add workspace.logFormat (default: unset)
│   └── templates/
│       └── deployment.yaml   # Inject AGENT_LOG_FORMAT env var when workspace.logFormat is set
└── deploy-operator-yaml/     # No change needed
```

**Structure Decision**: Existing single-project operator+agent layout. Changes span both binaries
and the Helm chart but follow established patterns without new packages or modules.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

*No exceptions required. Design is aligned with the constitution, with documentation gates still
pending implementation tasks in Phase 7.*
