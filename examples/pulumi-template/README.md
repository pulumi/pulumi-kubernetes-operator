# Template Examples

This directory contains examples of using `Template` to create dynamic Custom Resource Definitions (CRDs) backed by Pulumi infrastructure.

## Overview

`Template` is an **alternative approach** to the classic `Stack` + `Program` workflow. It allows platform engineers to define custom Kubernetes APIs that automatically provision cloud infrastructure when instances are created. This is similar to [kro (Kube Resource Orchestrator)](https://github.com/kubernetes-sigs/kro), but specifically designed for Pulumi-based infrastructure provisioning.

### Classic Approach vs Template

| Feature | Classic (Stack/Program) | Template |
|---------|------------------------|----------------|
| **Use case** | Direct infrastructure management | Platform engineering / self-service |
| **CRDs used** | `Stack`, `Program` | `Template` + generated CRDs |
| **User experience** | Users create Stack CRs | Users create custom resource instances (e.g., `Database`, `Bucket`) |
| **Abstraction level** | Full Pulumi control | Simplified, templated interface |
| **Best for** | DevOps teams, complex deployments | Platform teams providing golden paths |

**The classic approach is still fully supported and recommended** for:
- Direct Pulumi stack management
- Complex multi-stack deployments
- When you need full control over Pulumi configuration
- Existing workflows using `Stack` and `Program` CRs

**Use Template when you want to**:
- Create self-service infrastructure APIs for developers
- Abstract away Pulumi complexity behind simple custom resources
- Enforce organizational standards through templates

## How It Works

1. **Define a Template**: Create a `Template` resource that specifies:
   - The CRD to generate (API group, kind, schema)
   - The Pulumi resources to provision
   - How outputs map to status fields
   - Stack configuration (backend, secrets, service account)

2. **Operator Generates CRD**: When you apply the `Template`, the operator:
   - Validates the template
   - Generates a new CRD from the schema
   - Registers the CRD with the Kubernetes API server
   - Starts watching for instances of the new CRD

3. **Create Instances**: Users can now create instances of the generated CRD:
   ```yaml
   apiVersion: <your-group>/<version>
   kind: <YourKind>
   metadata:
     name: my-instance
   spec:
     # Fields defined in your template schema
   ```

4. **Infrastructure Provisioning**: For each instance:
   - The operator creates a `Program` CR with rendered resources
   - A `Stack` CR is created to run the Pulumi program
   - Outputs are copied to the instance's status

## Examples

### 1. Random Password Template (`random-password-template.yaml`)

A simple example that generates random passwords and stores them in Kubernetes Secrets.

**Generated CRD**: `randompasswords.secrets.example.com`

**Usage**:
```yaml
apiVersion: secrets.example.com/v1
kind: RandomPassword
metadata:
  name: my-app-password
spec:
  length: 24
  special: true
  secretKey: db-password
```

### 2. City Website Template (`city-website-template.yaml`)

A simple AWS S3 static website example that displays "Hello World, {city}!" with configurable styling.

**Generated CRD**: `citywebsites.websites.example.com`

**Usage**:
```yaml
apiVersion: websites.example.com/v1
kind: CityWebsite
metadata:
  name: tokyo-site
  namespace: template-test
spec:
  city: Tokyo
  title: "Hello World"
  bodyColor: "#f0f8ff"
  textColor: "#1a1a2e"
```

**What it creates**:
- S3 bucket configured for static website hosting
- Public access configuration
- index.html with customizable content

### 3. Database Template (`database-template.yaml`)

A more complex example that provisions AWS RDS database instances with security groups and connection secrets.

**Generated CRD**: `databases.platform.example.com`

**Usage**:
```yaml
apiVersion: platform.example.com/v1
kind: Database
metadata:
  name: my-app-db
spec:
  name: my-app-db
  engine: postgres
  instanceClass: db.t3.large
  multiAZ: true
```

## Prerequisites

1. **Pulumi Access Token**: Create a secret with your Pulumi access token:
   ```bash
   kubectl create secret generic pulumi-api-secret \
     --from-literal=accessToken=pul-xxxxx
   ```

2. **Service Account**: Create a service account for the Stack workspaces:
   ```bash
   kubectl create serviceaccount pulumi-stack-sa
   ```

3. **RBAC**: Grant necessary permissions to the service account.

## Schema Field Types

The `schema` section supports the following field types:

| Type | Description | Constraints |
|------|-------------|-------------|
| `string` | Text value | `minLength`, `maxLength`, `pattern`, `enum`, `format` |
| `integer` | Whole number | `minimum`, `maximum` |
| `number` | Floating point | `minimum`, `maximum` |
| `boolean` | True/false | - |
| `array` | List of items | `items` (nested schema) |
| `object` | Nested object | `properties`, `additionalProperties` |

## Expression Language

Resources and outputs use **Pulumi YAML native expressions** with `${...}` syntax:

- **Schema references**: `${schema.spec.fieldName}`, `${schema.metadata.name}`
- **Config references**: `${config.keyName}` (from `config` section)
- **Resource references**: `${resourceId.property}` (for outputs and cross-resource refs)
- **Conditional**: `${condition ? valueIfTrue : valueIfFalse}`
- **String concat**: `${value1 + "-" + value2}`

These expressions are compatible with Pulumi YAML programs. See the [Pulumi YAML documentation](https://www.pulumi.com/docs/languages-sdks/yaml/) for more details on expression syntax.

## Lifecycle Options

```yaml
lifecycle:
  destroyOnDelete: true      # Destroy infrastructure when CR is deleted
  refreshBeforeUpdate: true  # Run pulumi refresh before updates
  continueOnError: false     # Fail fast on errors
  protectResources: false    # Protect resources from accidental deletion
```

## CRD Specification Options

### Plural Name

By default, the plural name is computed as `lowercase(kind) + "s"`. For kinds that don't follow standard English pluralization, you can specify an explicit plural name:

```yaml
spec:
  crd:
    apiVersion: platform.example.com/v1
    kind: Policy
    plural: policies  # Instead of auto-generated "policys"
```

Common cases where explicit plural is useful:
- `Policy` → `policies` (not "policys")
- `Index` → `indices` (not "indexs")
- `Status` → `statuses` (not "statuss")
- `Entry` → `entries` (not "entrys")

### Other CRD Options

```yaml
spec:
  crd:
    apiVersion: platform.example.com/v1
    kind: Database
    plural: databases           # Optional: explicit plural name
    scope: Namespaced           # or Cluster
    categories:
      - pulumi
      - infrastructure
    shortNames:
      - db
    printerColumns:
      - name: Engine
        type: string
        jsonPath: .spec.engine
```

## Status Fields

Every generated CRD automatically includes these status fields:

- `conditions`: Standard Kubernetes conditions (Ready, Progressing, etc.)
- `stackRef`: Reference to the underlying Stack CR
- `lastUpdate`: Information about the last Pulumi update
- `observedGeneration`: Last processed generation
- `ready`: Boolean indicating if the instance is ready (Stack succeeded)

Plus any custom fields defined in `schema.status`.

## Troubleshooting

1. **CRD not created**: Check the Template status:
   ```bash
   kubectl get template -o yaml
   ```

2. **Instance not reconciling**: Check if Stack was created:
   ```bash
   kubectl get stack -l pulumi.com/template=<template-name>
   ```

3. **Pulumi errors**: Check the Update CR logs:
   ```bash
   kubectl get update -o yaml
   kubectl logs -l pulumi.com/workspace=<workspace-name>
   ```

## Observability

### Prometheus Metrics

The Template controller exposes Prometheus metrics for monitoring:

| Metric | Type | Description |
|--------|------|-------------|
| `templates_active` | Gauge | Number of Template CRs tracked |
| `templates_failing` | GaugeVec | Templates with failed reconciliation |
| `templates_reconciling` | GaugeVec | Templates currently reconciling |
| `template_instances_total` | GaugeVec | Instance count per Template |
| `template_reconcile_duration_seconds` | HistogramVec | Reconciliation duration |

### Structured Logging

All reconciliation logs include correlation IDs for tracing:
- `reconcileID`: Unique ID for each reconciliation
- `template`: Template namespace/name
- `instance`: Instance namespace/name (for instance operations)

## Implementation Notes

### Server-Side Apply (SSA)

The Template controller uses **Server-Side Apply** for creating Program and Stack CRs. This provides:

- **Field ownership tracking**: The controller owns fields it manages via `TemplateFieldManager`
- **GitOps compatibility**: Works alongside ArgoCD, Flux, and other tools
- **Conflict handling**: Graceful handling of concurrent updates

### Rate Limiting

The controller uses exponential backoff rate limiting:
- Base delay: 1 second
- Max delay: 5 minutes

This prevents overwhelming the API server during error scenarios.

## Security Considerations

The Template controller requires **wildcard RBAC permissions** (`*/*`) to manage dynamically generated CRDs. See [Security Considerations](../../docs/design/dynamic-crd-generation.md#security-considerations) in the design doc for:

- Understanding the security implications
- Mitigation strategies (network policies, audit logging, RBAC restrictions)
- How to disable the Template feature if not needed
