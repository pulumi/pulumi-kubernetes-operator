# Feature Specification: Workspace Agent Structured JSON Logging

**Feature Branch**: `001-workspace-json-logging`
**Created**: 2026-03-03
**Status**: Draft
**Input**: GitHub Issue #952 — Allow configure workspace to run pulumi with --json

## Context

This feature addresses two related but distinct logging surfaces in workspace pods:

1. **Agent logger format** — the zap-based structured logger used by the agent process itself
   (startup messages, errors, gRPC events, dependency installation output). Currently hardcoded
   to development/console format. This feature adds support for switching to JSON format via
   configuration, without changing the default. Console remains the default to preserve
   backward compatibility.

2. **Pulumi CLI output format** — the stdout/stderr emitted by `pulumi up/down/preview/refresh`
   commands, which the agent currently pipes through its zap logger line-by-line as plain text.
   The Pulumi Automation API already delivers structured engine events via the `EventStreams`
   channel (no `--json` flag needed). This feature exposes that capability as a configurable
   option.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Workspace Agent Can Be Configured to Emit Structured JSON Logs (Priority: P1)

An operator administrator deploys the Pulumi Kubernetes Operator and wants workspace pod logs
to be immediately parseable by log aggregation backends (OpenSearch, Loki, Elasticsearch, Datadog)
without custom parsing rules. Currently the workspace agent uses a human-readable console format
that these tools cannot parse automatically.

The operator exposes a configuration mechanism (`AGENT_LOG_FORMAT=json`, Workspace CR
`spec.logFormat`, or Helm `workspace.logFormat`) to opt into JSON structured logging. The console
format remains the default, preserving existing terminal-watching workflows.

**Why this priority**: Required for any user trying to integrate workspace pod logs with a log
aggregation backend. Console format is the root cause of the issue. Adding a JSON opt-in resolves
it for production users with zero risk to existing console-watching workflows.

**Independent Test**: Deploy the operator with `AGENT_LOG_FORMAT=json`, create any Stack, and
verify that every log line from the workspace pod is a valid JSON object parseable by `jq`.

**Acceptance Scenarios**:

1. **Given** the operator is deployed with `AGENT_LOG_FORMAT=json`,
   **When** a Workspace pod starts and the agent initialises,
   **Then** all agent log lines are valid JSON objects containing at minimum `level`, `ts`,
   and `msg` fields.

2. **Given** the agent is configured for JSON logs,
   **When** an error occurs during a Pulumi operation,
   **Then** the error appears as a structured JSON log entry with appropriate severity level,
   parseable by standard JSON tooling.

3. **Given** the agent is configured for JSON logs,
   **When** Pulumi dependency installation output is captured,
   **Then** it is wrapped as structured log entries in JSON format, not interleaved raw text.

---

### User Story 2 - Log Format Is Configurable; Console Remains the Default (Priority: P2)

A developer running the operator locally or a team with specific logging requirements uses the
default console format without any configuration. Teams that need structured logs for aggregation
backends opt in via environment variable, Workspace CR, or Helm value.

**Why this priority**: Console remains the default — no breaking change for existing deployments.
JSON is additive. The configuration mechanism must be consistent with the operator's existing
`controller.logFormat` pattern.

**Independent Test**: Deploy the operator with no log format configuration. Verify workspace pods
emit human-readable console logs. Then set `AGENT_LOG_FORMAT=json` and verify JSON output.

**Acceptance Scenarios**:

1. **Given** the operator is deployed with no log format configuration,
   **When** a Workspace pod starts,
   **Then** the agent emits human-readable console logs (default behaviour unchanged).

2. **Given** `AGENT_LOG_FORMAT` is set to an unrecognised value,
   **When** the agent starts,
   **Then** it fails immediately with a clear error message listing accepted values — it does not
   silently fall back.

3. **Given** a Workspace CR with `spec.logFormat: json`,
   **When** no cluster-level default is set,
   **Then** the agent for that workspace emits JSON logs (per-workspace override takes
   precedence over cluster default).

---

### User Story 3 - Helm Chart Exposes Workspace Log Format (Priority: P3)

A cluster administrator manages operator deployments via Helm and wants to configure the workspace
agent log format through the same values file used for the operator's own log format
(`controller.logFormat`), keeping configuration consistent and GitOps-compatible.

**Why this priority**: Without Helm support, teams managing clusters via GitOps must inject the
environment variable through manual patching, breaking the single-source-of-truth model. The
operator already has `controller.logFormat` — the workspace should follow the same pattern.

**Independent Test**: Run `helm template --set workspace.logFormat=json` and verify the rendered
controller Deployment includes `AGENT_LOG_FORMAT=json`. Then create a Workspace with
`spec.logFormat` unset and verify the resulting workspace pod receives that cluster default with
no CR changes required.

**Acceptance Scenarios**:

1. **Given** `workspace.logFormat` is not set in Helm values,
   **When** the chart is deployed,
   **Then** workspace pods use console log format (default unchanged).

2. **Given** `workspace.logFormat: json` in Helm values,
   **When** the chart is deployed,
   **Then** the controller Deployment receives `AGENT_LOG_FORMAT=json`, establishing the
   cluster-wide default that workspace pods inherit when `spec.logFormat` is unset.

---

### User Story 4 - Pulumi CLI Operations Emit Structured JSON Output (Priority: P4)

An advanced operator user wants not just the agent's own logs to be structured, but also the
output of Pulumi operations themselves (`pulumi up`, `pulumi destroy`, etc.) to be emitted as
machine-readable JSON engine events rather than human-readable progress text. This enables
downstream processing of resource-level change events from the workspace pod log stream.

**Why this priority**: This is a distinct, additive capability beyond the agent logger format.
The Pulumi Automation API already delivers structured `apitype.EngineEvent` objects via the
`EventStreams` channel (no `--json` CLI flag needed). No version gate is required. It is not
required to resolve the original issue but is the natural next step. When enabled, engine events
replace (not supplement) the `ProgressStreams` output in the pod log, giving clean structured
data without duplicate human-readable noise.

**Independent Test**: Can be fully tested by creating a Stack, running an update, and verifying
that engine event lines in the workspace pod log are structured JSON objects matching the Pulumi
engine event schema.

**Acceptance Scenarios**:

1. **Given** a Workspace or cluster configured for JSON Pulumi output,
   **When** a `pulumi up` operation executes,
   **Then** each engine event line in the workspace pod log is a valid JSON object with
   `type` and `sequence` fields matching the Pulumi engine event schema.

2. **Given** JSON Pulumi output is enabled,
   **When** a resource creation fails,
   **Then** the failure is captured as a structured engine event, not a plain-text error string.

3. **Given** JSON Pulumi output is not configured (default),
   **When** a `pulumi up` operation executes,
   **Then** operation output appears as human-readable text lines wrapped in agent log entries
   (existing behaviour, no regression).

---

### Edge Cases

- What happens if `AGENT_LOG_FORMAT` is set to an unrecognised value? The agent must fail fast
  with a descriptive error, not silently fall back to any format.
- What happens when a Workspace pod is upgraded in-place (StatefulSet rolling update)? Log format
  takes effect on the next pod restart — no Stack recreation required.
- How is Pulumi CLI stdout/stderr (currently piped via `zapio.Writer`) affected by the agent
  format switch? It must also respect the configured format, not bypass it.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The workspace agent MUST retain `console` as its default log output format.
  No configuration change is required for existing deployments — behaviour is unchanged.
- **FR-002**: The workspace agent MUST support an `AGENT_LOG_FORMAT` environment variable
  accepting `json` or `console` that overrides the default.
- **FR-003**: The workspace agent MUST fail to start with a clear error when `AGENT_LOG_FORMAT`
  is set to a value outside `json` and `console`.
- **FR-004**: The Workspace CR MUST expose an optional `spec.logFormat` field accepting `json`
  or `console` that overrides the environment variable for that specific workspace.
- **FR-005**: The WorkspaceReconciler MUST propagate Workspace CR fields to the workspace
  StatefulSet pod spec as environment variables: `spec.logFormat` → `AGENT_LOG_FORMAT` (when
  set), and `spec.pulumiJsonOutput` → `AGENT_PULUMI_JSON_OUTPUT=true` (only when the field is
  `true`). Unset `spec.logFormat` and `spec.pulumiJsonOutput: false` produce no env var injection,
  allowing the agent's built-in defaults to take effect.
- **FR-006**: The Helm chart MUST expose a `workspace.logFormat` value (default: unset, inheriting
  the agent's `console` default) that maps to the `AGENT_LOG_FORMAT` environment variable on the
  controller Deployment when set, establishing the cluster-wide default propagated to workspace
  pods by the WorkspaceReconciler.
- **FR-007**: Admission validation MUST reject Workspace CRs with `spec.logFormat` values
  outside of `["json", "console"]`.
- **FR-008**: Pulumi operation stdout/stderr (currently piped through the agent logger) MUST
  continue to be captured and emitted through the same structured log stream — not written raw
  to pod stdout bypassing the logger.
- **FR-009**: The log format MUST be settable without changes to individual Stack CRs — it is a
  workspace-level or cluster-level concern.
- **FR-010**: *(User Story 4)* The workspace agent MUST support an `AGENT_PULUMI_JSON_OUTPUT`
  environment variable (accepted values: `true` or `false`; default: `false`) and a
  `spec.pulumiJsonOutput` boolean Workspace CR field (default: `false`). When enabled: (a) logs each
  `apitype.EngineEvent` from the `EventStreams` channel as a structured log entry to the pod log,
  and (b) suppresses the `ProgressStreams` (`zapio.Writer`) channel so that human-readable
  progress text is not written alongside the structured events. When disabled (default), behaviour
  is unchanged: `ProgressStreams` writes to the pod log and engine events are delivered only to
  the gRPC caller.
- **FR-011**: *(User Story 4)* The workspace agent MUST fail to start with a clear error when
  `AGENT_PULUMI_JSON_OUTPUT` is set to a value outside `true` and `false`, consistent with the
  fail-fast rule in FR-003.

### Key Entities

- **Workspace CR**: Extended with optional `spec.logFormat` field (enum: `json`, `console`),
  and `spec.pulumiJsonOutput` (boolean, default `false`) for User Story 4.
- **Workspace StatefulSet pod spec**: Receives `AGENT_LOG_FORMAT` (and optionally
  `AGENT_PULUMI_JSON_OUTPUT`) environment variables sourced from the Workspace CR or cluster
  defaults.
- **Agent Process**: Reads format configuration at startup and initialises its zap logger and
  Automation API options accordingly.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Workspace agent pods emit valid, parseable JSON log lines when `AGENT_LOG_FORMAT=json`
  is set — verifiable by piping pod log output through `jq` with no parse errors.
- **SC-002**: Switching log format requires only an environment variable change or Workspace CR
  update and takes effect on the next pod restart; no code changes or Stack recreations needed.
- **SC-003**: Workspace CRs with invalid `spec.logFormat` values are rejected at admission time
  with a user-readable validation error identifying accepted values.
- **SC-004**: Existing deployments with no `AGENT_LOG_FORMAT` set continue to emit console logs
  with no changes needed — the default is unchanged.
- **SC-005**: New code paths are covered by tests at the appropriate level: agent format-switching
  logic by unit tests; WorkspaceReconciler env var propagation by controller unit/integration
  tests; Helm `workspace.logFormat` value by Helm template rendering tests (verify env var
  appears in rendered StatefulSet spec — no cluster required). Overall test coverage does not
  decrease.
- **SC-006**: CHANGELOG is updated; CRD documentation is regenerated to reflect schema changes;
  Helm chart README reflects the new `workspace.logFormat` value.
- **SC-007**: When `AGENT_PULUMI_JSON_OUTPUT=true`, each Pulumi operation produces at least one
  engine event log line parseable by `jq` containing `type` and `sequence` fields; no
  `ProgressStreams` human-readable text appears in the pod log for the same operation.

## Manual Smoke Test

A before/after Kind-cluster validation using a two-pet-name Pulumi YAML program (one plain
output, one secret output). Running a real Stack confirms both the log format change and that
secret values remain masked as `[secret]` in the new JSON output.

### Prerequisites

```bash
# Kind and kubectl must be installed
kind create cluster --name pko-log-test
kubectl cluster-info --context kind-pko-log-test
```

### Test Program

The Program CR creates two `random:index:RandomPet` resources. `publicPet` is a plain output;
`secretPet` is wrapped with `fn::secret`, making it a Pulumi secret. This lets us verify that
the secret value never appears in plaintext in the pod logs regardless of format.

```yaml
# program.yaml
apiVersion: pulumi.com/v1
kind: Program
metadata:
  name: two-pets
  namespace: default
program:
  resources:
    petA:
      type: random:index:RandomPet
    petB:
      type: random:index:RandomPet
  outputs:
    publicPet: ${petA.id}
    secretPet:
      fn::secret: ${petB.id}
```

The Stack CR references the Program and uses a local file backend with a passphrase secrets
provider to avoid requiring Pulumi Cloud credentials:

```yaml
# stack.yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: two-pets
  namespace: default
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: two-pets:system:auth-delegator
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:auth-delegator
subjects:
- kind: ServiceAccount
  name: two-pets
  namespace: default
---
apiVersion: pulumi.com/v1
kind: Stack
metadata:
  name: two-pets
  namespace: default
spec:
  serviceAccountName: two-pets
  programRef:
    name: two-pets
  stack: dev
  destroyOnFinalize: true
  envs:
  - PULUMI_BACKEND_URL=file:///tmp/pulumi-state
  - PULUMI_CONFIG_PASSPHRASE=smoke-test
```

### Step 1 — Install and Baseline (Console Format)

Install the operator and CRDs from the current branch (before any code changes):

```bash
make install-crds
cd operator && make run
```

Deploy the program and stack:

```bash
kubectl apply -f program.yaml
kubectl apply -f stack.yaml
```

Wait for the workspace pod to start:

```bash
kubectl wait pod -l pulumi.com/workspace=two-pets --for=condition=Ready --timeout=120s
```

Capture logs and confirm console format:

```bash
kubectl logs -l pulumi.com/workspace=two-pets | head -30
```

Expected baseline (console format — NOT parseable by `jq`):

```
2026-03-10T12:00:00.000Z  INFO  agent started  {"version": "..."}
2026-03-10T12:00:00.100Z  INFO  installing dependencies
```

Confirm console logs fail JSON parsing:

```bash
kubectl logs -l pulumi.com/workspace=two-pets | jq .
# Expected: parse error — confirms console format is active
```

### Step 2 — After: Opt In to JSON Format

Apply the feature changes, then enable JSON logging on the Stack's workspace:

```bash
# Option A: per-workspace via CR field (after implementing spec.logFormat)
kubectl patch stack two-pets --type=merge \
  -p '{"spec":{"workspaceTemplate":{"spec":{"logFormat":"json"}}}}'

# Option B: cluster-wide via operator env var
kubectl set env deployment/controller-manager AGENT_LOG_FORMAT=json \
  -n pulumi-kubernetes-operator
```

Restart the workspace pod to apply:

```bash
kubectl delete pod -l pulumi.com/workspace=two-pets
kubectl wait pod -l pulumi.com/workspace=two-pets --for=condition=Ready --timeout=120s
```

Verify all log lines parse as JSON:

```bash
# Zero errors expected
kubectl logs -l pulumi.com/workspace=two-pets | jq .

# Confirm required fields on every line
kubectl logs -l pulumi.com/workspace=two-pets | jq '{level, ts, msg}' | head -20
```

Expected output after the change:

```json
{"level":"info","ts":"2026-03-10T12:01:00.000Z","msg":"agent started","version":"..."}
{"level":"info","ts":"2026-03-10T12:01:00.100Z","msg":"installing dependencies"}
```

Verify the secret pet name is masked and never appears in plaintext:

```bash
# secretPet value must appear only as [secret], never as the actual pet name
kubectl logs -l pulumi.com/workspace=two-pets | grep -i "secret"
# Expected: lines show "[secret]" placeholder, not the resolved pet name string
```

### Step 3 — Validate Invalid Value Is Rejected

```bash
kubectl patch workspace two-pets --type=merge -p '{"spec":{"logFormat":"invalid"}}'
# Expected: admission webhook rejects with a validation error listing accepted values
```

### Teardown

```bash
kubectl delete -f stack.yaml
kubectl delete -f program.yaml
kind delete cluster --name pko-log-test
```

## Assumptions

- `json` and `console` are the only two supported log format values, consistent with the
  operator's existing `--zap-encoder` convention and Helm chart (`controller.logFormat`).
- `console` remains the default log format. JSON is opt-in. This is not a breaking change.
- User Story 4 (Pulumi CLI JSON output) requires no Automation API version gate. The
  `EventStreams` channel (already wired for all operations in `server.go`) delivers structured
  `apitype.EngineEvent` objects natively — the `--json` CLI flag is irrelevant when using the
  Automation API. The stretch-goal designation reflects scope priority, not technical risk.
- Log level configuration (`--verbose`) is out of scope for this feature — only format is
  addressed.
- No changes to the Stack CR API are required; this is a workspace-layer concern.
- Secret values are **never logged in plaintext** by the agent. The Pulumi CLI masks secrets
  before they reach the `zapio.Writer` pipe (emitting `[secret]` placeholders). Stack outputs
  carry a `Secret` boolean at the gRPC protocol level (`marshalOutputs`) but secret values
  remain encrypted. The `--show-secrets` flag is NOT used anywhere in the operator. No additional
  redaction layer is needed; implementers MUST NOT introduce `--show-secrets` as part of this
  feature.

## Clarifications

### Session 2026-03-10

- Q: Should the agent apply redaction to log field values when emitting structured JSON? →
  A: Redact only fields already marked sensitive in existing code. Confirmed by code review:
  Pulumi CLI masks secrets upstream of the logger; no `--show-secrets` flag is used; the
  existing `Secret` boolean on config/output values is sufficient.
- Q: What is the minimum Pulumi Automation API version required for structured engine event output
  (FR-010)? → A: Already supported — current version — no gate needed. The `EventStreams` channel
  already used in `server.go` delivers native `apitype.EngineEvent` structs; the `--json` CLI
  flag is bypassed entirely by the Automation API.
- Q: Should the operator emit a migration signal for existing deployments when the default log
  format changes? → A: Do not change the default — console remains default; only add JSON support
  as an opt-in. No migration signal needed; no breaking change introduced.
- Q: When `AGENT_PULUMI_JSON_OUTPUT` is enabled, what should the agent do with operation output? →
  A: Log engine events to the pod log AND suppress `ProgressStreams`. Structured `EventStreams`
  data replaces human-readable `ProgressStreams` (zapio.Writer) output in the pod log; engine
  events continue to the gRPC caller regardless.
- Q: What test level is required for the Helm chart `workspace.logFormat` value? →
  A: Helm template unit test only. Verify env var appears in rendered StatefulSet spec; no
  cluster required. Agent behavior covered by agent unit tests.
- Q: What values should `AGENT_PULUMI_JSON_OUTPUT` accept? →
  A: `true` / `false` strings; fail fast on any other value. Consistent with FR-003 fail-fast
  pattern. FR-011 added to capture the validation rule.
- Q: Should a success criterion be added for FR-010 engine event logging? →
  A: Yes — SC-007 added: engine event lines parseable by `jq` with `type`/`sequence` fields;
  no `ProgressStreams` text present in pod log when enabled.
- Q: Should FR-005 be extended or a new FR-012 added for `spec.pulumiJsonOutput` propagation? →
  A: Extend FR-005. Both CR fields follow the same reconciler propagation pattern; FR-005 now
  covers `spec.logFormat` → `AGENT_LOG_FORMAT` and `spec.pulumiJsonOutput` →
  `AGENT_PULUMI_JSON_OUTPUT`, with unset fields producing no injection.
