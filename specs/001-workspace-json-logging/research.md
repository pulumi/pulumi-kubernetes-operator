# Research: Workspace Agent Structured JSON Logging

## R-001: Zap Logger Configuration (Console vs JSON)

**Decision**: Switch logger encoding by reading `AGENT_LOG_FORMAT` env var before `zc.Build()` in
`agent/cmd/root.go`'s `PersistentPreRunE`.

**Current state** (`agent/cmd/root.go`):
```go
zc := zap.NewDevelopmentConfig()   // Encoding = "console", EncodeTime = ISO8601
zc.DisableCaller = true
zc.DisableStacktrace = true
if !verbose {
    zc.Level.SetLevel(zap.InfoLevel)
}
zapLog, err := zc.Build()
```

**Implementation**: Two approaches considered:

Option A — Flip only the `Encoding` field on `zap.NewDevelopmentConfig()`:
```go
if format == "json" {
    zc.Encoding = "json"
    zc.EncoderConfig = zap.NewProductionEncoderConfig()
}
```
Preserves existing dev config (DisableCaller, DisableStacktrace) while switching format.

Option B — Branch between `zap.NewDevelopmentConfig()` and `zap.NewProductionConfig()`:
```go
var zc zap.Config
if format == "json" {
    zc = zap.NewProductionConfig()
} else {
    zc = zap.NewDevelopmentConfig()
}
```
Cleaner; production config already emits JSON with RFC3339Nano timestamps and structured fields.

**Rationale**: Option B is preferred. `zap.NewProductionConfig()` produces standard JSON with
`level`, `ts` (Unix epoch float64), and `msg` fields — exactly what the spec requires (SC-001
checks for these fields via `jq`). The `DisableCaller` and `DisableStacktrace` adjustments can
be applied to both branches. Console default behaviour is completely unchanged.

**Key difference between configs**:
| Field | Development (console) | Production (json) |
|-------|----------------------|-------------------|
| Encoding | `"console"` | `"json"` |
| TimeKey | `"T"` (ISO8601) | `"ts"` (Unix epoch float64) |
| LevelKey | `"L"` | `"level"` |
| MessageKey | `"M"` | `"msg"` |
| CallerKey | `"C"` | `"caller"` |
| Sampling | disabled | enabled (100 initial, 100/s thereafter) |

The spec requires `level`, `ts`, and `msg` — production config provides these exactly.
Sampling on the production config should be disabled (or set to nil) to avoid dropping agent
startup or error messages. Final choice: `zap.NewProductionConfig()` with `Config.Sampling = nil`.

**Alternatives considered**: `zapcore.NewConsoleEncoder` vs `zapcore.NewJSONEncoder` selection via
`zap.RegisterEncoder` — rejected as more complex with no benefit over config-based switching.

---

## R-002: Fail-Fast Validation on AGENT_LOG_FORMAT

**Decision**: Read `AGENT_LOG_FORMAT` at `PersistentPreRunE` entry, validate before any logger
is built, return error immediately for unrecognised values.

**Pattern** (consistent with existing `--auth-mode` validation in `serve.go`):
```go
format := os.Getenv("AGENT_LOG_FORMAT")
switch format {
case "", "console":
    // default: console
case "json":
    // json
default:
    return fmt.Errorf("unsupported AGENT_LOG_FORMAT %q: accepted values are \"json\", \"console\"", format)
}
```

**Rationale**: The spec (FR-003) requires a clear error on unrecognised values. Returning an error
from `PersistentPreRunE` causes cobra to print the error and exit non-zero — immediately visible
in pod logs and pod `lastState.terminated.reason`.

---

## R-003: Workspace CR Field Design — LogFormat

**Decision**: New named type `LogFormat string` with kubebuilder validation markers.

```go
// LogFormat specifies the log output format for the workspace agent.
// +enum
type LogFormat string

const (
    LogFormatConsole LogFormat = "console"
    LogFormatJSON    LogFormat = "json"
)
```

Added to `WorkspaceSpec`:
```go
// LogFormat controls the output format of the workspace agent's logger.
// Accepted values: "json", "console". Default (unset): agent uses its built-in default (console).
// +kubebuilder:validation:Enum=json;console
// +optional
LogFormat LogFormat `json:"logFormat,omitempty"`
```

**Rationale**: Using a named type with `+enum` and `+kubebuilder:validation:Enum` generates CRD
schema validation (XValidation on the field via `enum` in OpenAPI). This enforces FR-007 at the
Kubernetes API server level without requiring a webhook. Consistent with `SecurityProfile` which
uses the same pattern in the same file. The existing `SecurityProfile` type (line 31 of
`workspace_types.go`) is the direct template.

---

## R-004: Workspace CR Field Design — PulumiJsonOutput

**Decision**: Optional `*bool` field on `WorkspaceSpec`. Using `*bool` (pointer) to distinguish
"unset" (no env var injection) from explicitly `false` (inject `AGENT_PULUMI_JSON_OUTPUT=false`).
However, since "unset" and `false` produce the same agent behaviour, a plain `bool` with
`omitempty` is sufficient and simpler.

```go
// PulumiJsonOutput enables structured JSON engine event output from Pulumi operations.
// When true, engine events are logged to the pod log and ProgressStreams is suppressed.
// Default (unset/false): ProgressStreams is active and engine events are not logged.
// +optional
PulumiJsonOutput bool `json:"pulumiJsonOutput,omitempty"`
```

No enum marker needed (boolean). `+kubebuilder:validation` not required beyond the field type.
`AGENT_PULUMI_JSON_OUTPUT` validation (`true`/`false` strings; fail-fast on other values) is
handled identically to `AGENT_LOG_FORMAT` in `root.go`.

---

## R-005: Env Var Propagation — WorkspaceReconciler

**Decision**: Extend `newStatefulSet()` in `workspace_controller.go` to inject env vars from CR
fields, with operator-level cluster defaults as fallback.

**Precedence** (highest to lowest):
1. `w.Spec.LogFormat` CR field → injects `AGENT_LOG_FORMAT=<value>`
2. Operator env var `AGENT_LOG_FORMAT` (set via Helm `workspace.logFormat`) → injects cluster default
3. Nothing injected → agent uses built-in default (console)

**Implementation** (after existing `AGENT_MEMLIMIT` injection):
```go
// Inject AGENT_LOG_FORMAT: CR field takes precedence over cluster default
if w.Spec.LogFormat != "" {
    env = append(env, corev1.EnvVar{
        Name:  "AGENT_LOG_FORMAT",
        Value: string(w.Spec.LogFormat),
    })
} else if clusterDefault := os.Getenv("AGENT_LOG_FORMAT"); clusterDefault != "" {
    env = append(env, corev1.EnvVar{
        Name:  "AGENT_LOG_FORMAT",
        Value: clusterDefault,
    })
}

// Inject AGENT_PULUMI_JSON_OUTPUT if enabled at workspace level
if w.Spec.PulumiJsonOutput {
    env = append(env, corev1.EnvVar{
        Name:  "AGENT_PULUMI_JSON_OUTPUT",
        Value: "true",
    })
}
```

**Rationale**: Follows the `AGENT_IMAGE` / `AGENT_IMAGE_PULL_POLICY` pattern already established
in `agentImage()` (line 519-523 of `workspace_controller.go`). No new operator flags needed.
The cluster-level default is set on the operator manager pod (Helm injects it) and read by the
operator process via `os.Getenv`. This is idempotent and respects workspace stability (Principle
IV): changing the env var causes a StatefulSet spec update which triggers rolling pod replacement
— the expected behaviour.

---

## R-006: Helm Chart Changes

**Decision**: Add `workspace.logFormat` as an optional string value (unset by default). When set,
inject `AGENT_LOG_FORMAT` env var into the controller manager deployment (not directly into
workspace pods — the controller reads it and propagates it to workspace StatefulSets).

**values.yaml addition** (after `controller:` section):
```yaml
workspace:
  # -- Log format for workspace agent pods (one of 'json' or 'console').
  # When unset, workspace agents use their built-in default (console).
  # Corresponds to the AGENT_LOG_FORMAT environment variable.
  logFormat: ""
```

**templates/deployment.yaml addition** (after existing `AGENT_IMAGE_PULL_POLICY` env var):
```yaml
{{- if .Values.workspace.logFormat }}
- name: AGENT_LOG_FORMAT
  value: {{ .Values.workspace.logFormat }}
{{- end }}
```

**Rationale**: Mirrors the `controller.logFormat` pattern (`--zap-encoder={{ .Values.controller.logFormat }}`).
`workspace.logFormat` is separate from `controller.logFormat` because they configure different
processes (controller manager vs workspace agent). The empty-string default ensures no env var is
injected when unset, preserving the agent's console default.

---

## R-007: FR-010 — PulumiJsonOutput Engine Event Logging in server.go

**Decision**: Add `PulumiJsonOutput bool` to `server.Options`. When enabled, remove
`ProgressStreams` and `ErrorProgressStreams` from automation options and instead log engine events
(already delivered via `EventStreams`) to the pod log using `s.plog`.

**Current pattern** (each of Up/Down/Preview/Refresh operations):
```go
stdout := &zapio.Writer{Log: s.plog, Level: zap.InfoLevel}
defer stdout.Close()
stderr := &zapio.Writer{Log: s.plog, Level: zap.WarnLevel}
defer stderr.Close()
opts = append(opts, optup.ProgressStreams(stdout))
opts = append(opts, optup.ErrorProgressStreams(stderr))

events := make(chan events.EngineEvent)
opts = append(opts, optup.EventStreams(events))
go func() {
    for evt := range events {
        data, err := marshalEngineEvent(evt.EngineEvent)
        // ... send to gRPC caller
    }
}()
```

**When `PulumiJsonOutput == true`**: Skip `ProgressStreams` and `ErrorProgressStreams`; in the
`EventStreams` goroutine, additionally log each event to the pod log:
```go
if s.pulumiJsonOutput {
    s.plog.Info("engine event", zap.Any("event", data))
}
```

The `marshalEngineEvent` function already converts `apitype.EngineEvent` to `*structpb.Struct`,
which can be logged as a structured zap field. Engine events continue to flow to the gRPC caller
regardless (no change to the gRPC streaming path).

**Rationale**: `EventStreams` is already wired for all four operations. The Automation API delivers
`apitype.EngineEvent` structs natively — no `--json` CLI flag needed. Suppressing
`ProgressStreams` ensures no duplicate human-readable + structured output in the pod log when JSON
engine events are active (SC-007).

**AGENT_PULUMI_JSON_OUTPUT validation** (in `root.go`): Same fail-fast pattern as
`AGENT_LOG_FORMAT`. The parsed boolean is passed to `server.Options.PulumiJsonOutput`.

---

## R-008: Test Strategy

| Layer | What to test | Tool |
|-------|-------------|------|
| Agent — logger init | `AGENT_LOG_FORMAT=json` → production config; `AGENT_LOG_FORMAT=badvalue` → error | `go test` unit |
| Agent — server.go | `PulumiJsonOutput=true` → no ProgressStreams; engine events logged | `go test` unit |
| WorkspaceReconciler | `spec.logFormat` → env var in StatefulSet pod spec; cluster default via os.Getenv | `envtest` controller test |
| WorkspaceReconciler | `spec.pulumiJsonOutput=true` → `AGENT_PULUMI_JSON_OUTPUT=true` in pod spec | `envtest` controller test |
| CRD admission | `spec.logFormat: badvalue` → rejected by API server schema validation | `envtest` (apply with invalid value) |
| Helm template | `workspace.logFormat=json` → `AGENT_LOG_FORMAT=json` in rendered deployment.yaml | `helm template` unit render test |

No new E2E tests required beyond existing suite — functional correctness covered by unit/controller
tests. Manual smoke test (spec section) validates end-to-end on Kind cluster.
