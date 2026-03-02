# Data Model: Workspace Agent Structured JSON Logging

## Entity: WorkspaceSpec (extended)

**File**: `operator/api/auto/v1alpha1/workspace_types.go`
**Change type**: Additive (two new optional fields)

### New Type: LogFormat

```go
// LogFormat specifies the log output format for the workspace agent.
// +enum
type LogFormat string

const (
    // LogFormatConsole emits human-readable console logs (zap development format).
    LogFormatConsole LogFormat = "console"
    // LogFormatJSON emits structured JSON logs (zap production format).
    LogFormatJSON LogFormat = "json"
)
```

### New Fields on WorkspaceSpec

```go
// LogFormat controls the output format of the workspace agent's logger.
// When set, overrides the cluster-level AGENT_LOG_FORMAT environment variable.
// When unset, the agent uses its built-in default (console format).
// +kubebuilder:validation:Enum=json;console
// +optional
LogFormat LogFormat `json:"logFormat,omitempty"`

// PulumiJsonOutput enables structured JSON engine event output from Pulumi operations.
// When true: engine events from the Automation API EventStreams channel are logged to the
// pod log as structured JSON; ProgressStreams (human-readable text) is suppressed.
// When false or unset: ProgressStreams is active and engine events are not logged to the pod.
// +optional
PulumiJsonOutput bool `json:"pulumiJsonOutput,omitempty"`
```

### Field Semantics

| Field | Type | Default | Env var set on pod | Precedence |
|-------|------|---------|-------------------|------------|
| `spec.logFormat` | `LogFormat` (string enum) | `""` (unset) | `AGENT_LOG_FORMAT=<value>` | Highest: overrides cluster default |
| `spec.pulumiJsonOutput` | `bool` | `false` | `AGENT_PULUMI_JSON_OUTPUT=true` (only when `true`) | Per-workspace only |

### Validation Rules

- `spec.logFormat`: Accepted values `"json"`, `"console"`, or unset (`""`).
  Enforced by CRD schema (`+kubebuilder:validation:Enum`) — rejected by the Kubernetes API server
  before the request reaches the controller.
- `spec.pulumiJsonOutput`: Boolean — no enum validation needed.

---

## Entity: Server.Options (extended)

**File**: `agent/pkg/server/server.go`
**Change type**: Additive (one new field)

```go
type Options struct {
    StackName       string
    SecretsProvider string
    PulumiLogLevel  uint
    // PulumiJsonOutput enables structured JSON engine event output from Pulumi operations.
    // When true: logs engine events via the pod logger; suppresses ProgressStreams output.
    PulumiJsonOutput bool
}
```

**Server struct** (new field):
```go
type Server struct {
    workspace      auto.Workspace
    plog           *zap.Logger
    stack          *auto.Stack
    pulumiLogLevel uint
    // pulumiJsonOutput controls whether Pulumi operation events are logged as JSON
    pulumiJsonOutput bool
}
```

---

## Configuration Flow (State Transitions)

```
Helm values.yaml
  workspace.logFormat = "json"
      │
      ▼
Controller Manager Deployment (env var)
  AGENT_LOG_FORMAT = "json"
      │
      ▼ (os.Getenv in newStatefulSet when spec.logFormat unset)
Workspace StatefulSet pod spec
  env[AGENT_LOG_FORMAT] = "json"
      │
      ▼ (agent reads at startup)
zap global logger
  Encoding = "json"

─────────────────────────────────────────────────────

Workspace CR
  spec.logFormat = "json"           (per-workspace override)
      │
      ▼ (WorkspaceReconciler.newStatefulSet)
Workspace StatefulSet pod spec
  env[AGENT_LOG_FORMAT] = "json"    (takes precedence over cluster default)
      │
      ▼
zap global logger
  Encoding = "json"

─────────────────────────────────────────────────────

Workspace CR
  spec.pulumiJsonOutput = true
      │
      ▼ (WorkspaceReconciler.newStatefulSet)
Workspace StatefulSet pod spec
  env[AGENT_PULUMI_JSON_OUTPUT] = "true"
      │
      ▼ (agent reads at startup → server.Options.PulumiJsonOutput = true)
server.go Up/Down/Preview/Refresh
  ProgressStreams: nil (suppressed)
  EventStreams: logs engine events via s.plog + sends to gRPC caller
```

---

## Env Var Contract

| Variable | Accepted values | Default (unset) | Read by | Set by |
|----------|----------------|-----------------|---------|--------|
| `AGENT_LOG_FORMAT` | `"json"`, `"console"` | `"console"` (built-in) | Agent `cmd/root.go` | WorkspaceReconciler or operator env |
| `AGENT_PULUMI_JSON_OUTPUT` | `"true"`, `"false"` | `"false"` (built-in) | Agent `cmd/root.go` | WorkspaceReconciler |

Both variables follow the fail-fast rule: any value outside the accepted set causes the agent
process to exit with a non-zero status and a descriptive error message.

---

## CRD Schema Impact

The following CRD manifest will change:
- `deploy/crds/auto.pulumi.com_workspaces.yaml`
- `deploy/helm/pulumi-operator/crds/auto.pulumi.com_workspaces.yaml`

New schema properties under `spec`:
```yaml
logFormat:
  description: |
    LogFormat controls the output format of the workspace agent's logger.
    When set, overrides the cluster-level AGENT_LOG_FORMAT environment variable.
  enum:
  - json
  - console
  type: string
pulumiJsonOutput:
  description: |
    PulumiJsonOutput enables structured JSON engine event output from Pulumi operations.
  type: boolean
```

Regeneration command: `make generate-crds` (runs `cd operator && make manifests` then copies to
`deploy/crds/` and `deploy/helm/pulumi-operator/crds/`).
