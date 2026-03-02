# Quickstart: Workspace Agent Structured JSON Logging

## Enable JSON Logging — Cluster-Wide (Helm)

```yaml
# values-override.yaml
workspace:
  logFormat: json
```

```bash
helm upgrade pulumi-kubernetes-operator pulumi/pulumi-kubernetes-operator \
  -f values-override.yaml
```

All new workspace pods will emit JSON logs. Existing pods pick up the change on their next
restart (e.g., after a Stack or Workspace CR update triggers pod replacement).

---

## Enable JSON Logging — Per Workspace (CR)

```yaml
apiVersion: auto.pulumi.com/v1alpha1
kind: Workspace
metadata:
  name: my-workspace
  namespace: default
spec:
  logFormat: json
  # ... other fields
```

This workspace pod will emit JSON logs regardless of cluster-wide settings.

---

## Enable Structured Pulumi Engine Event Output

```yaml
apiVersion: auto.pulumi.com/v1alpha1
kind: Workspace
metadata:
  name: my-workspace
  namespace: default
spec:
  pulumiJsonOutput: true
```

When `pulumi up` runs, each engine event appears as a structured JSON line in the pod log:

```json
{"level":"info","ts":"2026-03-10T12:00:01Z","logger":"pulumi","msg":"engine event","type":"resourcePreEvent","sequence":1}
{"level":"info","ts":"2026-03-10T12:00:02Z","logger":"pulumi","msg":"engine event","type":"resOutputsEvent","sequence":2}
```

Human-readable progress text is suppressed when `pulumiJsonOutput: true`.

---

## Verify JSON Format

```bash
# Check all lines parse as valid JSON (zero errors = success)
kubectl logs -l pulumi.com/workspace=my-workspace | jq .

# Confirm required fields on every line
kubectl logs -l pulumi.com/workspace=my-workspace | jq '{level, ts, msg}' | head -10
```

Expected output (production zap uses Unix epoch float for `ts`):
```json
{"level":"info","ts":1741608000.123,"msg":"Pulumi Kubernetes Agent","version":"2.x.x"}
{"level":"info","ts":1741608000.456,"msg":"opened a local workspace","workspace":"/share/workspace","project":"my-project","runtime":"nodejs"}
```

---

## Validate Invalid Values Are Rejected

```bash
kubectl patch workspace my-workspace --type=merge -p '{"spec":{"logFormat":"invalid"}}'
# Expected: error from server: Workspace.auto.pulumi.com "my-workspace" is invalid:
#   spec.logFormat: Unsupported value: "invalid": supported values: "json", "console"
```

---

## Revert to Console (Default)

Remove the field or set it to `"console"`:

```bash
kubectl patch workspace my-workspace --type=merge -p '{"spec":{"logFormat":"console"}}'
# or remove the field:
kubectl patch workspace my-workspace --type=json -p '[{"op":"remove","path":"/spec/logFormat"}]'
```

Format takes effect on the next pod restart.
