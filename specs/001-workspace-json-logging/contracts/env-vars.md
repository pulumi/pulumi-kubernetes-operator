# Contract: Environment Variable Interface

## AGENT_LOG_FORMAT

**Type**: String
**Accepted values**: `"json"`, `"console"`
**Default (unset)**: `"console"` (agent built-in default)
**Read by**: `agent/cmd/root.go` `PersistentPreRunE`
**Set by**: WorkspaceReconciler `newStatefulSet()` (from `spec.logFormat` or cluster default)

### Behaviour

| Value | Logger encoding | Time format | Level key |
|-------|----------------|-------------|-----------|
| `""` (unset) or `"console"` | `console` (zap dev) | ISO8601 | `L` |
| `"json"` | `json` (zap prod, sampling disabled) | RFC3339Nano | `level` |

Invalid value → agent exits with:
```
unsupported AGENT_LOG_FORMAT "badvalue": accepted values are "json", "console"
```

### Precedence (highest to lowest)

1. `spec.logFormat` CR field → injected as `AGENT_LOG_FORMAT` by WorkspaceReconciler
2. `AGENT_LOG_FORMAT` on controller manager pod (Helm `workspace.logFormat`) →
   read by WorkspaceReconciler via `os.Getenv`, injected when `spec.logFormat` is unset
3. Unset → agent uses console default (no env var injection)

---

## AGENT_PULUMI_JSON_OUTPUT

**Type**: String (boolean)
**Accepted values**: `"true"`, `"false"`
**Default (unset)**: `"false"` (agent built-in default)
**Read by**: `agent/cmd/root.go` `PersistentPreRunE`
**Set by**: WorkspaceReconciler `newStatefulSet()` when `spec.pulumiJsonOutput == true`

### Behaviour

| Value | ProgressStreams | EventStreams pod logging | gRPC EventStreams |
|-------|---------------|------------------------|-------------------|
| `""` (unset) or `"false"` | Active (zapio.Writer → plog) | Not logged to pod | Active (existing) |
| `"true"` | Suppressed (nil) | Logged as structured JSON | Active (existing) |

When `"true"`: each `apitype.EngineEvent` from the Automation API is logged to the pod log as a
structured zap entry with fields matching the engine event schema (`type`, `sequence`, etc.).

Invalid value → agent exits with:
```
unsupported AGENT_PULUMI_JSON_OUTPUT "badvalue": accepted values are "true", "false"
```
