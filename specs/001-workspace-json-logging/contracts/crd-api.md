# Contract: Workspace CRD API

## New Fields: auto.pulumi.com/v1alpha1 Workspace

### spec.logFormat

```yaml
spec:
  logFormat: "json"   # or "console", or omit for default
```

- **Type**: `string` (enum)
- **Enum values**: `["json", "console"]`
- **Default**: unset (agent uses console built-in default)
- **Validation**: Enforced by CRD schema — the Kubernetes API server rejects values outside the
  enum before the request reaches the controller
- **Effect**: The WorkspaceReconciler injects `AGENT_LOG_FORMAT=<value>` into the workspace pod
  StatefulSet when this field is set. Takes precedence over the cluster-level default.

### spec.pulumiJsonOutput

```yaml
spec:
  pulumiJsonOutput: true   # or false (default)
```

- **Type**: `boolean`
- **Default**: `false` (unset / omitted)
- **Validation**: Boolean — no enum required
- **Effect**: When `true`, the WorkspaceReconciler injects `AGENT_PULUMI_JSON_OUTPUT=true` into
  the workspace pod StatefulSet. The agent enables structured engine event logging and suppresses
  human-readable ProgressStreams output.

---

## Helm Values API

### workspace.logFormat

```yaml
workspace:
  logFormat: "json"   # or "console", or leave empty (default)
```

- **Type**: `string`
- **Default**: `""` (unset — no env var injected into controller manager)
- **Maps to**: `AGENT_LOG_FORMAT` env var on the controller manager deployment
- **Effect**: When non-empty, all workspace pods (that don't override via `spec.logFormat`) use
  this format. The controller manager reads `AGENT_LOG_FORMAT` from its own environment and
  propagates it to workspace StatefulSets during reconciliation.
- **Override**: `spec.logFormat` on individual Workspace CRs takes precedence.

---

## Backward Compatibility

All new fields are optional with safe defaults:
- `spec.logFormat` omitted → no env var injected → agent uses console (unchanged)
- `spec.pulumiJsonOutput` omitted/false → no env var injected → ProgressStreams active (unchanged)
- `workspace.logFormat` unset in Helm → no env var on controller manager (unchanged)

No existing Workspace CRs require modification. No existing deployments change behaviour.
