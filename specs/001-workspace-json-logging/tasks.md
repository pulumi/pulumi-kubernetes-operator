# Tasks: Workspace Agent Structured JSON Logging

**Feature branch**: `001-workspace-json-logging`
**Input**: Design documents from `/specs/001-workspace-json-logging/`
**Spec**: [spec.md](spec.md) | **Plan**: [plan.md](plan.md) | **Data model**: [data-model.md](data-model.md)
**Contracts**: [contracts/env-vars.md](contracts/env-vars.md) | [contracts/crd-api.md](contracts/crd-api.md)

**Organization**: Tasks grouped by user story — each story is independently implementable and testable.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel with other [P] tasks in the same phase (different files, no unmet deps)
- **[Story]**: User story label (`[US1]`–`[US4]`)
- Tests are included per SC-005 requirement — TDD order: tests first, then implementation

---

## Phase 1: Setup

**Purpose**: Verify the starting state is clean before changes land.

- [X] T001 Verify clean baseline — run `make build && make lint` from repo root and confirm zero errors

**Checkpoint**: Build and lint pass. Ready to add types.

---

## Phase 2: Foundational — CRD Type Definitions

**Purpose**: Define the new `WorkspaceSpec` fields and regenerate all derived artifacts. Both US2
(per-workspace log format) and US4 (Pulumi JSON output) depend on these types being present before
their controller and agent code can be implemented or tested.

**⚠️ CRITICAL**: US2 and US4 cannot begin until this phase is complete.

- [X] T002 Add `LogFormat` string enum type with `LogFormatConsole`/`LogFormatJSON` constants and add `LogFormat` (with `+kubebuilder:validation:Enum=json;console`) and `PulumiJsonOutput bool` fields to `WorkspaceSpec` in `operator/api/auto/v1alpha1/workspace_types.go`
- [X] T003 Regenerate DeepCopy methods and apply configurations with `cd operator && make generate` (updates `operator/api/auto/v1alpha1/zz_generated_deepcopy.go` and `operator/internal/apply/`)
- [X] T004 Regenerate CRD manifests with `make generate-crds` to update `deploy/crds/auto.pulumi.com_workspaces.yaml` and `deploy/helm/pulumi-operator/crds/auto.pulumi.com_workspaces.yaml`

**Checkpoint**: CRD schema includes `spec.logFormat` enum and `spec.pulumiJsonOutput` boolean;
DeepCopy and apply configs are in sync. User story phases can now begin independently.

---

## Phase 3: User Story 1 — Agent JSON Log Format (Priority: P1) 🎯 MVP

**Goal**: The workspace agent reads `AGENT_LOG_FORMAT` at startup. When set to `json`, it
initialises `zap.NewProductionConfig()` (with sampling disabled); when unset or `console`, it
keeps `zap.NewDevelopmentConfig()`. Any other value causes immediate fail-fast exit.

**Independent Test**: Set `AGENT_LOG_FORMAT=json`, start the agent, verify every log line is a
valid JSON object with `level`, `ts`, and `msg` fields parseable by `jq`.

### Tests for User Story 1

> **Write first — ensure they FAIL before implementing T006**

- [X] T005 [US1] Add unit tests for `AGENT_LOG_FORMAT` in `agent/cmd/root_test.go`: (a) unset → development/console config; (b) `json` → production config with sampling nil; (c) `console` → development config; (d) invalid value → error containing `"accepted values are"` string

### Implementation for User Story 1

- [X] T006 [US1] Implement `AGENT_LOG_FORMAT` env var reading with fail-fast switch validation and `zap.NewProductionConfig()`/`zap.NewDevelopmentConfig()` branching (set `Config.Sampling = nil` for json branch) in `agent/cmd/root.go` `PersistentPreRunE`

**Checkpoint**: `AGENT_LOG_FORMAT=json` → all agent log lines are valid JSON. T005 passes.

---

## Phase 4: User Story 2 — Per-Workspace CR Configuration (Priority: P2)

**Goal**: A Workspace CR with `spec.logFormat: json` causes the WorkspaceReconciler to inject
`AGENT_LOG_FORMAT=json` into the workspace StatefulSet pod spec. The CR field takes precedence
over the cluster-level `AGENT_LOG_FORMAT` env var on the controller manager. CRD schema validation
rejects values outside `["json", "console"]`.

**Independent Test**: Apply a Workspace CR with `spec.logFormat: json`; verify the workspace pod's
env vars include `AGENT_LOG_FORMAT=json`. Apply with `spec.logFormat: invalid`; verify admission
rejection.

### Tests for User Story 2

> **Write first — ensure they FAIL before implementing T008**

- [X] T007 [US2] Add envtest controller tests in `operator/internal/controller/auto/workspace_controller_test.go`: (a) `spec.logFormat: json` → `AGENT_LOG_FORMAT=json` in StatefulSet pod spec; (b) `spec.logFormat` unset + `os.Getenv("AGENT_LOG_FORMAT")="json"` → env var injected from cluster default; (c) both unset → no `AGENT_LOG_FORMAT` env var; (d) invalid `spec.logFormat` value rejected by CRD admission; (e) repeated reconciliation does not duplicate env vars or change deterministic ordering

### Implementation for User Story 2

- [X] T008 [US2] Extend `newStatefulSet()` in `operator/internal/controller/auto/workspace_controller.go` to inject `AGENT_LOG_FORMAT` — CR field takes precedence over `os.Getenv("AGENT_LOG_FORMAT")` cluster default; unset fields produce no injection (follow `AGENT_IMAGE` pattern at lines 519–523)

**Checkpoint**: Workspace CR `spec.logFormat` drives the pod env var. T007 passes.

---

## Phase 5: User Story 3 — Helm Chart Workspace Log Format (Priority: P3)

**Goal**: Cluster administrators set `workspace.logFormat: json` in Helm values. When non-empty,
the chart injects `AGENT_LOG_FORMAT=<value>` into the controller manager deployment env, which
establishes the cluster default the WorkspaceReconciler propagates to workspace pods when
`spec.logFormat` is unset.

**Independent Test**: Run `helm template` with `workspace.logFormat=json` and verify `AGENT_LOG_FORMAT: json` appears under `env:` in the rendered Deployment manifest. Then verify, via the controller test path, that a Workspace with `spec.logFormat` unset inherits that cluster default into its pod env.

### Tests for User Story 3

> **Write first — ensure they FAIL before implementing T010/T011**

- [X] T009 [US3] Add Helm template rendering test in `deploy/helm/pulumi-operator/tests/workspace_log_format_test.sh` (create file if absent; follow existing patterns in `deploy/helm/`): (a) run `helm template deploy/helm/pulumi-operator/ --set workspace.logFormat=json | yq eval '.spec.template.spec.containers[0].env[] | select(.name == "AGENT_LOG_FORMAT") | .value'` and assert output equals `json`; (b) run same without `--set` and assert `AGENT_LOG_FORMAT` is absent from output; (c) document that workspace pod propagation is verified by the US2 controller test covering cluster-default injection

### Implementation for User Story 3

- [X] T010 [P] [US3] Add `workspace.logFormat: ""` string value with YAML doc comment (`# -- Log format for workspace agent pods (one of 'json' or 'console'). Corresponds to AGENT_LOG_FORMAT.`) to `deploy/helm/pulumi-operator/values.yaml`
- [X] T011 [P] [US3] Add conditional env var template block `{{- if .Values.workspace.logFormat }} - name: AGENT_LOG_FORMAT ... {{- end }}` after `AGENT_IMAGE_PULL_POLICY` in `deploy/helm/pulumi-operator/templates/deployment.yaml`

**Checkpoint**: `helm template --set workspace.logFormat=json` renders `AGENT_LOG_FORMAT: json` in controller Deployment, and the controller test path proves that default reaches workspace pods when `spec.logFormat` is unset. T009 passes.

---

## Phase 6: User Story 4 — Pulumi CLI Structured Engine Event Output (Priority: P4)

**⚠️ PREREQUISITE: User Story 1 (Phase 3) MUST be complete before any US4 tasks begin.** T012
adds tests to `agent/cmd/root_test.go` — the same file that T005 (US1) already modifies. T016
sets `server.Options.PulumiJsonOutput`, a field that T015 creates. Starting US4 before US1 is
fully merged will cause root_test.go conflicts and compile failures.

**Goal**: When `AGENT_PULUMI_JSON_OUTPUT=true`, the agent suppresses `ProgressStreams` /
`ErrorProgressStreams` (human-readable text) and instead logs each `apitype.EngineEvent` from
`EventStreams` as a structured zap entry to the pod log. Engine events continue flowing to the
gRPC caller unchanged. The CR field `spec.pulumiJsonOutput: true` drives injection of
`AGENT_PULUMI_JSON_OUTPUT=true` into the pod spec.

**Independent Test**: Apply a Workspace with `spec.pulumiJsonOutput: true`, run a Stack update,
verify pod log lines contain `type` and `sequence` fields; confirm no human-readable ProgressStreams
text appears for the same operation.

### Tests for User Story 4

> **Write first — ensure they FAIL before implementing T015–T018**

- [X] T012 [P] [US4] Add unit tests for `AGENT_PULUMI_JSON_OUTPUT` in `agent/cmd/root_test.go`: (a) unset → false; (b) `"true"` → true; (c) `"false"` → false; (d) invalid value → fail-fast error containing `"accepted values are"`
- [X] T013 [P] [US4] Add unit test in `agent/pkg/server/server_test.go`: `Options{PulumiJsonOutput: true}` → Up/Down/Preview/Refresh builds nil ProgressStreams, logs engine events through `plog` as JSON containing `type` and `sequence`, and emits no human-readable progress text for the same operation; `PulumiJsonOutput: false` → ProgressStreams active + engine events not logged to pod
- [X] T014 [P] [US4] Add envtest controller test in `operator/internal/controller/auto/workspace_controller_test.go`: `spec.pulumiJsonOutput: true` → `AGENT_PULUMI_JSON_OUTPUT=true` in StatefulSet pod spec; `spec.pulumiJsonOutput: false` → env var absent; repeated reconciliation does not duplicate env vars or change deterministic ordering

### Implementation for User Story 4

- [X] T015 [P] [US4] Add `PulumiJsonOutput bool` field to `Options` struct and `pulumiJsonOutput bool` field to `Server` struct in `agent/pkg/server/server.go`; set `s.pulumiJsonOutput` in `New()` from options
- [X] T016 [US4] Add `AGENT_PULUMI_JSON_OUTPUT` env var reading with fail-fast switch validation (`"true"` / `"false"` / `""` accepted; any other → error) to `agent/cmd/root.go` `PersistentPreRunE`; pass parsed bool as `server.Options.PulumiJsonOutput` (depends on T015 — server.Options.PulumiJsonOutput must exist first)
- [X] T017 [US4] Implement conditional streaming options in `agent/pkg/server/server.go` Up/Down/Preview/Refresh: when `pulumiJsonOutput=true` suppress `ProgressStreams`/`ErrorProgressStreams` and log engine events via `s.plog` as JSON entries that preserve `type` and `sequence`; when false preserve existing behaviour (depends on T015; choose either inline or helper-based wiring, but keep all four operations consistent)
- [X] T018 [US4] Extend `newStatefulSet()` in `operator/internal/controller/auto/workspace_controller.go` to inject `AGENT_PULUMI_JSON_OUTPUT=true` when `spec.pulumiJsonOutput == true`; unset/false produces no injection (sequential after T008 — same function)

**Checkpoint**: `spec.pulumiJsonOutput: true` → pod log shows structured engine event JSON, no ProgressStreams text. T012, T013, T014 pass.

---

## Phase 7: Polish & Cross-Cutting Concerns

- [X] T019 [P] **[MANDATORY]** Update `CHANGELOG.md` with user-facing changes: new `spec.logFormat`, `spec.pulumiJsonOutput` CRD fields; `workspace.logFormat` Helm value; `AGENT_LOG_FORMAT` / `AGENT_PULUMI_JSON_OUTPUT` env vars
- [X] T020 [P] **[MANDATORY — CRD CHANGED]** Regenerate CRD documentation with `make generate-crdocs` to reflect `spec.logFormat` and `spec.pulumiJsonOutput` in `docs/workspaces.md`
- [X] T021 [P] Run `make lint` across all modified files and fix any `golangci-lint` / `gosec` / `goheader` issues
- [X] T022 [P] Run `ct lint` and `ah lint` on `deploy/helm/pulumi-operator/` to verify Helm chart compliance (`ct lint --chart-dirs deploy/helm --charts deploy/helm/pulumi-operator`)
- [X] T023 [P] Update Helm chart README with new `workspace.logFormat` value — run `helm-docs` in `deploy/helm/pulumi-operator/` or manually update `deploy/helm/pulumi-operator/README.md` to document the new value, its defaults, and correspondence to `AGENT_LOG_FORMAT`
- [X] T024 Run `make test` (operator + agent) and verify all unit and controller tests pass
- [X] T025 Run `make test-e2e` against a Kind cluster to execute the formal E2E suite on the Stack→Workspace→Update→Agent workflow and verify no regressions
- [X] T026 Run quickstart.md smoke test steps against a Kind cluster (`kind create cluster --name pko-log-test`) to validate end-to-end JSON log format switching, secret masking, and that no Stack CR changes are required (verify the Stack CR spec is unmodified after workspace logFormat configuration takes effect)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 — **BLOCKS all user stories**
- **US1 (Phase 3)**: Depends only on Phase 2 (agent changes are independent of CRD types)
- **US2 (Phase 4)**: Depends on Phase 2 (needs `LogFormat` type in workspace_types.go)
- **US3 (Phase 5)**: Depends on Phase 2 (conceptually); Helm test is independent of US2
- **US4 (Phase 6)**: Depends on Phase 2 **and US1 fully complete** (root_test.go and root.go must be in final state); T018 sequential after T008 (same function)
- **Polish (Phase 7)**: Depends on all desired user story phases being complete

### User Story Dependencies

| Story | Blocking deps | Notes |
|-------|--------------|-------|
| US1 (P1) | Phase 2 | Pure agent change — no controller dependency |
| US2 (P2) | Phase 2 | WorkspaceSpec type must exist |
| US3 (P3) | Phase 2; US2 complete for full e2e | Helm test is independent of US2 |
| US4 (P4) | Phase 2 **+ US1 complete** | T016 depends on T015; T018 sequential after T008 |

### Within Each User Story

- Tests MUST be written and confirmed failing before implementation
- For US4: T015 (server.go struct) before T016 (root.go, uses Options.PulumiJsonOutput) and T017 (server.go methods)
- For US4: T008 (AGENT_LOG_FORMAT injection) before T018 (AGENT_PULUMI_JSON_OUTPUT injection) — both modify `newStatefulSet()`

### Parallel Opportunities

- All user story phases can start in parallel after Phase 2 completes (except US4 which also requires US1)
- Within US3: T010 (values.yaml) and T011 (deployment.yaml) are parallel [P] — different files
- Within US4 tests: T012, T013, T014 are parallel [P] — different files
- Within US4 implementation: T015 (server.go struct) is parallel [P] with T012/T013/T014; T016 and T017 follow T015 sequentially
- Polish: T019, T020, T021, T022, T023 are parallel [P]

---

## Parallel Example: User Story 4 Tests

```bash
# Prerequisite: US1 (Phase 3) complete.
# Then launch all US4 tests in parallel:
Task T012: "Add AGENT_PULUMI_JSON_OUTPUT unit tests in agent/cmd/root_test.go"
Task T013: "Add PulumiJsonOutput server unit test in agent/pkg/server/server_test.go"
Task T014: "Add envtest controller test in workspace_controller_test.go"
# All three are in different files — no conflicts

# Then implement server.go struct first (no deps within US4):
Task T015: "Add PulumiJsonOutput field to server.go structs" [P — different file from test files]

# Then T016 and T017 sequentially (both depend on T015):
Task T016: "Add AGENT_PULUMI_JSON_OUTPUT parsing in agent/cmd/root.go" (depends T015)
Task T017: "Implement conditional streaming in server.go" (depends T015)
# T018 follows T008 in the same controller function
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL — blocks all stories)
3. Complete Phase 3: User Story 1 (agent reads env var, emits JSON logs)
4. **STOP and VALIDATE**: `AGENT_LOG_FORMAT=json` → `kubectl logs | jq .` succeeds
5. This resolves the core issue (#952) with minimal scope

### Incremental Delivery

1. Phase 1 + Phase 2 → CRD types + DeepCopy + manifests ready
2. **US1** → JSON logs via env var (MVP — resolves #952)
3. **US2** → Per-workspace override via `spec.logFormat` CR field
4. **US3** → Helm `workspace.logFormat` for GitOps-compatible cluster defaults
5. **US4** → Structured Pulumi engine event output (stretch goal)
6. Polish → CHANGELOG, CRD docs, Helm README, lint, tests, e2e

### Single-Developer Sequential Order

```
T001 → T002 → T003 → T004 → T005 → T006 → T007 → T008 → T009 → T010 → T011
     → T012 → T013 → T014 → T015 → T016 → T017 → T018
     → T019 → T020 → T021 → T022 → T023 → T024 → T025 → T026
```

---

## Notes

- `[P]` tasks have no file conflicts with other `[P]` tasks in the same phase
- `[Story]` label maps each task to its user story for traceability
- Each user story phase is independently completable and testable before the next begins
- T003 (`make generate`) MUST happen BEFORE T004 (`make generate-crds`) — both follow T002
- T016 (root.go AGENT_PULUMI_JSON_OUTPUT) depends on T015 (server.go struct) — NOT parallelizable
- T017 (server.go streaming logic): either inline or helper-based wiring is acceptable; helper reduces 4x repetition but adds indirection
- Secret masking is NOT modified: Pulumi CLI already masks secrets upstream of `zapio.Writer`
- `--show-secrets` MUST NOT be introduced anywhere in this implementation
- FR-009 (log format settable without Stack CR changes) is verified by the T026 smoke test note; no dedicated unit test is needed since it is a design property of the architecture
