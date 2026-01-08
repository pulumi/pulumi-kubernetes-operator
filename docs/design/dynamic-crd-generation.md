# Dynamic CRD Generation for Pulumi Kubernetes Operator

## Overview

This document describes the dynamic Custom Resource Definition (CRD) generation capability added to the Pulumi Kubernetes Operator, inspired by [kro (Kube Resource Orchestrator)](https://github.com/kubernetes-sigs/kro). This feature enables platform engineers to define custom Kubernetes APIs that provision cloud infrastructure through Pulumi, without writing controller code.

## Status

**Implementation Status: Complete (v1alpha1)**

The feature has been implemented and tested. Key components:
- `Template` CRD: `pulumi.com/v1alpha1`
- Controller: `TemplateReconciler`
- Dynamic CRD generation and instance watching
- Full lifecycle management (create, update, delete with destroy)

## Motivation

### Current State

The Pulumi Kubernetes Operator provides:
- **Stack CR**: Represents a Pulumi stack deployment
- **Program CR**: Inline YAML-based Pulumi program definition
- **Workspace CR**: Execution environment for Pulumi operations
- **Update CR**: Represents a single Pulumi operation

While powerful, users must work directly with these CRDs and understand Pulumi concepts. Platform teams often want to expose simplified, domain-specific APIs to their developers.

### Desired State

Platform engineers should be able to:
1. Define a **Template** that specifies a custom API schema and underlying Pulumi resources
2. Have the operator automatically generate a new CRD from this template
3. Allow developers to create instances of the generated CRD without Pulumi knowledge
4. The operator handles the full lifecycle: creating Stacks, running updates, syncing status

### Use Cases

1. **Self-Service Infrastructure**: Platform team creates a `Database` CRD; developers request databases without knowing about RDS, CloudSQL, or Pulumi
2. **Standardized Environments**: Create a `DevEnvironment` CRD that provisions VPCs, EKS clusters, and databases with guardrails
3. **Multi-Cloud Abstraction**: Define `StorageBucket` that works across AWS S3, GCP GCS, and Azure Blob
4. **Composable Building Blocks**: Chain multiple Templates for complex infrastructure patterns

## Design

### Template CRD

The `Template` CRD is defined in `operator/api/pulumi/v1alpha1/template_types.go`:

```yaml
apiVersion: pulumi.com/v1alpha1
kind: Template
metadata:
  name: city-website
  namespace: template-test
spec:
  # The CRD that will be generated
  crd:
    apiVersion: websites.example.com/v1
    kind: CityWebsite
    scope: Namespaced
    categories:
      - websites
    shortNames:
      - cw
      - cityweb
    printerColumns:
      - name: City
        type: string
        jsonPath: .spec.city
      - name: WebsiteURL
        type: string
        jsonPath: .status.websiteUrl

  # Schema for the generated CRD instances
  schema:
    spec:
      city:
        type: string
        description: "Name of the city to display"
        required: true
      title:
        type: string
        description: "Title of the website"
        default: "Hello World"

    status:
      bucketName:
        type: string
        description: "Name of the S3 bucket"
      websiteUrl:
        type: string
        description: "URL of the website"

  # Pulumi resources that will be created for each instance
  resources:
    website-bucket:
      type: aws:s3:Bucket
      properties:
        bucket: ${schema.metadata.namespace}-${schema.metadata.name}-website
        website:
          indexDocument: index.html

    index-html:
      type: aws:s3:BucketObject
      properties:
        bucket: ${website-bucket.id}
        key: index.html
        content: |
          <h1>${schema.spec.title}, ${schema.spec.city}!</h1>
        contentType: text/html

  # Variables for intermediate expressions
  variables:
    bucketArn: ${website-bucket.arn}

  # Outputs mapped to instance status
  outputs:
    bucketName: ${website-bucket.id}
    websiteUrl: ${website-bucket.websiteEndpoint}

  # Stack configuration
  stackConfig:
    serviceAccountName: pulumi-stack-sa
    envRefs:
      PULUMI_ACCESS_TOKEN:
        type: Secret
        secret:
          name: pulumi-api-secret
          key: accessToken
    # Pulumi ESC environment support
    environment:
      - pulumi-idp/auth

  # Lifecycle settings
  lifecycle:
    destroyOnDelete: true
    refreshBeforeUpdate: false
```

### Generated CRD Example

When the above `Template` is applied, the operator generates:

```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: citywebsites.websites.example.com
  labels:
    pulumi.com/template-name: city-website
    pulumi.com/template-namespace: template-test
spec:
  group: websites.example.com
  names:
    kind: CityWebsite
    listKind: CityWebsiteList
    plural: citywebsites
    singular: citywebsite
    shortNames:
      - cw
      - cityweb
    categories:
      - websites
  scope: Namespaced
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              required:
                - city
              properties:
                city:
                  type: string
                  description: Name of the city to display
                title:
                  type: string
                  default: "Hello World"
                  description: Title of the website
            status:
              type: object
              properties:
                bucketName:
                  type: string
                websiteUrl:
                  type: string
                ready:
                  type: boolean
                stackRef:
                  type: string
                conditions:
                  type: array
                  items:
                    type: object
                    # ... standard condition fields
      subresources:
        status: {}
      additionalPrinterColumns:
        - name: Ready
          type: boolean
          jsonPath: .status.ready
        - name: City
          type: string
          jsonPath: .spec.city
        - name: WebsiteURL
          type: string
          jsonPath: .status.websiteUrl
        - name: Age
          type: date
          jsonPath: .metadata.creationTimestamp
```

### User Creates Instance

```yaml
apiVersion: websites.example.com/v1
kind: CityWebsite
metadata:
  name: berlin
  namespace: template-test
spec:
  city: Berlin
  title: Hello World
```

### What Happens Behind the Scenes

1. **TemplateReconciler** watches `Template` CRs
2. On create/update, it:
   - Validates the template (schema, expressions, resource references)
   - Generates CRD from schema
   - Registers CRD with API server
   - Creates a dynamic informer for the new CRD
3. **TemplateReconciler** watches instances of generated CRDs via dynamic informers
4. On instance create/update:
   - Adds finalizer for cleanup
   - Renders Pulumi program by substituting `${schema.*}` expressions
   - Creates/updates Program CR with rendered resources
   - Creates/updates Stack CR referencing the Program
   - Monitors Stack status and syncs to instance status
   - Copies outputs to instance status fields
5. On instance delete:
   - If `destroyOnDelete: true`, waits for Stack to destroy resources
   - Removes finalizer and allows instance deletion

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         Pulumi Kubernetes Operator                       │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌────────────────────────────────────────────────────────────────────┐ │
│  │                      TemplateReconciler                             │ │
│  │                                                                     │ │
│  │  - Watches Template CRs                                            │ │
│  │  - Validates schema                                                │ │
│  │  - Generates and registers CRDs                                    │ │
│  │  - Starts dynamic informers for generated CRDs                     │ │
│  │  - Handles instance lifecycle (create/update/delete)               │ │
│  │  - Creates Program and Stack CRs for each instance                 │ │
│  │  - Syncs status from Stack to instance                             │ │
│  └───────────┬────────────────────────────────────────────────────────┘ │
│              │                                                           │
│              │ creates/manages                                           │
│              ▼                                                           │
│  ┌────────────────────────┐    ┌────────────────────────────────────┐  │
│  │   Generated CRDs       │    │   Program CR + Stack CR            │  │
│  │                        │    │                                    │  │
│  │  - CityWebsite         │    │   (existing operator flow)         │  │
│  │  - Database            │    │                                    │  │
│  │  - S3Bucket            │    │   Stack → Workspace → Update       │  │
│  │  - ...                 │    │                                    │  │
│  └────────────────────────┘    └────────────────────────────────────┘  │
│                                                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

## Expression Language

### Simple Substitution

The Template uses a simple `${...}` expression syntax for value substitution:

```yaml
resources:
  website-bucket:
    type: aws:s3:Bucket
    properties:
      # Reference to schema fields
      bucket: ${schema.metadata.namespace}-${schema.metadata.name}-website

  index-html:
    type: aws:s3:BucketObject
    properties:
      # Reference to another resource
      bucket: ${website-bucket.id}
      # Reference to schema spec fields
      content: |
        <h1>${schema.spec.title}, ${schema.spec.city}!</h1>
```

### Reference Types

1. **`schema.spec.*`**: User-provided configuration from instance spec
2. **`schema.metadata.*`**: Instance metadata (name, namespace, labels, annotations)
3. **`<resourceId>.*`**: References to other resources in the template (passed through to Pulumi)

### Default Value Handling

Complex default value expressions using `||` are supported:

```yaml
properties:
  bucket: ${schema.spec.bucketName || schema.metadata.name}
```

This resolves to `schema.spec.bucketName` if provided, otherwise falls back to `schema.metadata.name`.

## Implementation Details

### File Structure

```
operator/
├── api/pulumi/v1alpha1/
│   └── template_types.go          # Template CRD type definitions
├── internal/controller/pulumi/
│   ├── template_controller.go     # Main controller implementation
│   ├── template_controller_test.go
│   ├── metrics_template.go        # Prometheus metrics for Templates
│   └── metrics_template_test.go
└── config/crd/bases/
    └── pulumi.com_templates.yaml  # Generated CRD manifest
```

### Server-Side Apply (SSA)

The Template controller uses **Server-Side Apply** for managing Program and Stack CRs, following Kubernetes 2025 best practices:

```go
// Use Server-Side Apply to create or update the Program
if err := r.Patch(ctx, program, client.Apply, client.FieldOwner(TemplateFieldManager)); err != nil {
    return nil, fmt.Errorf("failed to apply Program: %w", err)
}
```

**Benefits:**
- Proper field ownership tracking via `TemplateFieldManager`
- GitOps compatibility (operator owns secondaries, GitOps tools can own CR specs)
- No read-modify-write race conditions
- Graceful conflict handling

### Prometheus Metrics

The controller exposes the following Prometheus metrics:

| Metric | Type | Description |
|--------|------|-------------|
| `templates_active` | Gauge | Number of Template CRs currently tracked |
| `templates_failing` | GaugeVec | Templates where last reconcile failed |
| `templates_reconciling` | GaugeVec | Templates currently being reconciled |
| `template_instances_total` | GaugeVec | Number of instances per Template |
| `template_reconcile_duration_seconds` | HistogramVec | Duration of reconciliations by result |

These metrics are automatically registered with the controller-runtime metrics server and can be scraped by Prometheus.

### Key Components

#### TemplateSpec Fields

| Field | Type | Description |
|-------|------|-------------|
| `crd` | `CRDSpec` | Defines the CRD to generate (apiVersion, kind, scope, etc.) |
| `schema` | `TemplateSchema` | OpenAPI schema for spec and status fields |
| `resources` | `map[string]Resource` | Pulumi resources to create |
| `variables` | `map[string]Expression` | Intermediate expressions |
| `outputs` | `map[string]Expression` | Outputs mapped to status |
| `stackConfig` | `StackConfiguration` | Stack settings (serviceAccount, envRefs, environment) |
| `lifecycle` | `LifecycleConfig` | Destroy on delete, refresh settings |

#### CRDSpec Fields

| Field | Type | Description |
|-------|------|-------------|
| `apiVersion` | `string` | API group/version (e.g., `platform.example.com/v1`) |
| `kind` | `string` | Resource kind name |
| `plural` | `string` | Plural name (optional, defaults to kind + "s") |
| `scope` | `CRDScope` | `Namespaced` or `Cluster` |
| `categories` | `[]string` | kubectl categories |
| `shortNames` | `[]string` | Short aliases |
| `printerColumns` | `[]PrinterColumn` | Additional kubectl columns |

#### StackConfiguration Fields

| Field | Type | Description |
|-------|------|-------------|
| `serviceAccountName` | `string` | ServiceAccount for workspace pod |
| `backend` | `string` | Pulumi state backend URL |
| `secretsProvider` | `string` | Secrets encryption provider |
| `envRefs` | `map[string]ResourceRef` | Environment variable references |
| `environment` | `[]string` | Pulumi ESC environments |
| `workspaceTemplate` | `*EmbeddedWorkspaceTemplateSpec` | Custom workspace settings |

### Controller Flow

1. **Template Reconciliation** (`template_controller.go:115-231`)
   - Fetch Template CR
   - Handle deletion with finalizer
   - Validate template spec
   - Generate and apply CRD
   - Start dynamic informer for instances
   - Update Template status

2. **Instance Reconciliation** (`template_controller.go:700-780`)
   - Ensure finalizer on instance
   - Substitute expressions in resources
   - Create/update Program CR
   - Create/update Stack CR
   - Sync status from Stack to instance

3. **Status Synchronization** (`template_controller.go:1244-1358`)
   - Re-fetch instance to avoid conflicts
   - Map Stack conditions to instance
   - Copy outputs to status fields
   - Read secret outputs from Secret

### Labels and Ownership

Generated CRDs are labeled with:
- `pulumi.com/template-name`: Name of the Template
- `pulumi.com/template-namespace`: Namespace of the Template

Instance-related resources (Program, Stack) are labeled with:
- `pulumi.com/generated-by`: `pulumi-operator`
- `pulumi.com/template-name`: Template name
- `pulumi.com/template-namespace`: Template namespace
- `pulumi.com/instance-name`: Instance name
- `pulumi.com/instance-namespace`: Instance namespace

## Example: City Website Template

See `examples/pulumi-template/city-website-template.yaml` for a complete example that:
1. Creates an S3 bucket with static website hosting
2. Configures public access and bucket policy
3. Uploads an index.html with configurable city name
4. Exposes website URL in instance status

Usage:
```bash
# Apply the template
kubectl apply -f examples/pulumi-template/city-website-template.yaml

# Create instances
kubectl apply -f - <<EOF
apiVersion: websites.example.com/v1
kind: CityWebsite
metadata:
  name: berlin
  namespace: template-test
spec:
  city: Berlin
EOF

# Check status
kubectl get citywebsites -n template-test
# NAME     READY   CITY    WEBSITEURL
# berlin   True    Berlin  template-test-berlin-website.s3-website-us-east-1.amazonaws.com
```

## Backward Compatibility

The Template feature is **additive** and does not affect existing functionality:

- **Program CR**: Works as before for inline Pulumi programs
- **Stack CR**: Works as before with `programRef` or other sources
- **Workspace CR**: Unchanged
- **Update CR**: Unchanged

Users can continue using the existing Program/Stack workflow independently of Templates.

## Status Tracking

### Template Status

```yaml
status:
  conditions:
    - type: Ready
      status: "True"
      reason: ReconcileSucceeded
      message: "Template is ready"
  observedGeneration: 1
  crd:
    name: citywebsites.websites.example.com
  lastReconciled: "2024-01-15T10:30:00Z"
```

### Instance Status

```yaml
status:
  # User-defined outputs (from template.spec.outputs)
  bucketName: template-test-berlin-website
  websiteUrl: template-test-berlin-website.s3-website-us-east-1.amazonaws.com

  # All stack outputs
  outputs:
    bucketName: template-test-berlin-website
    websiteUrl: template-test-berlin-website.s3-website-us-east-1.amazonaws.com
    bucketEndpoint: http://template-test-berlin-website.s3-website-us-east-1.amazonaws.com

  # Standard fields (auto-injected)
  ready: true
  stackRef: city-website-berlin
  observedGeneration: 1
  conditions:
    - type: Ready
      status: "True"
      reason: StackSucceeded
      message: "Pulumi stack deployed successfully"
  lastUpdate:
    state: succeeded
    generation: 1
```

## Security Considerations

### RBAC Permissions (Critical)

The Template controller requires **wildcard RBAC permissions** to manage dynamically generated CRDs:

```yaml
- apiGroups:
  - '*'
  resources:
  - '*'
  verbs:
  - create
  - delete
  - get
  - list
  - patch
  - update
  - watch
```

**Security Implications:**
- The operator has cluster-wide access to all resources
- A compromised operator could affect any resource in the cluster
- This is necessary because the CRDs are user-defined at runtime

**Mitigation Strategies:**
1. **Network Policies**: Restrict network access to the operator pods
2. **Pod Security**: Run the operator with minimal privileges (non-root, read-only filesystem where possible)
3. **Audit Logging**: Enable Kubernetes audit logging to track operator actions
4. **RBAC for Templates**: Restrict who can create/modify Template CRs using standard Kubernetes RBAC
5. **Namespace Isolation**: Consider running separate operator instances per-namespace if stricter isolation is needed

**Disabling the Template Feature:**
If you don't need dynamic CRD generation, you can disable the Template controller by not registering it in the operator startup. This removes the need for wildcard permissions.

### Other Security Considerations

1. **CRD Naming**: Generated CRDs use user-specified API groups; operators should ensure groups don't conflict with system CRDs

2. **Expression Safety**: Simple string substitution only; no arbitrary code execution

3. **Secret Access**: Templates can reference secrets via `envRefs`; honor RBAC on referenced secrets

4. **Resource Limits**: Consider namespace quotas to prevent runaway resource creation

5. **API Group Collisions**: Prevent Templates from generating CRDs that could overwrite or shadow system CRDs (e.g., `apps`, `core`, `apiextensions.k8s.io`)

## Known Limitations

1. **Schema Evolution**: Changing Template schema after instances exist requires careful migration

2. **Single Stack per Instance**: Each instance creates one Stack; multi-stack support is not implemented

3. **Cross-Template Dependencies**: Templates cannot directly reference outputs from other Templates

4. **No Admission Webhooks**: Generated CRDs do not have validating webhooks beyond OpenAPI schema

## Future Enhancements

1. **CEL Expression Support**: Full CEL expressions for complex conditional logic
2. **Template Library**: Catalog of common templates
3. **Multi-Stack Templates**: Single instance creating multiple Stacks
4. **Cross-Template References**: Sharing outputs between Templates
5. **Cost Estimation**: Preview infrastructure costs before creation
6. **Validation Webhooks**: Custom validating admission webhooks for generated CRDs
7. **OpenTelemetry Tracing**: Distributed tracing for complex reconciliation flows
8. **Graceful Degradation**: Condition-based progress tracking for partial failures

### Already Implemented

- **Server-Side Apply (SSA)**: Implemented for Program and Stack management
- **Prometheus Metrics**: Full observability with 5 metrics + reconciliation duration histogram
- **Structured Logging**: Correlation IDs for tracing related operations
- **Rate Limiting**: Exponential backoff (1s base, 5min max)

## Comparison with kro

| Aspect | kro | Pulumi Operator Template |
|--------|-----|--------------------------|
| **Purpose** | Kubernetes resource composition | Cloud infrastructure provisioning |
| **Resources** | Kubernetes objects only | Any Pulumi provider (AWS, GCP, Azure, K8s, etc.) |
| **Expression Language** | CEL | Pulumi YAML expressions (CEL planned) |
| **Execution** | Direct K8s API calls | Pulumi Automation API via workspace pods |
| **State Management** | Kubernetes etcd | Pulumi state backend (S3, GCS, Pulumi Cloud) |
| **Drift Detection** | K8s informers | Pulumi refresh |
| **Dependencies** | DAG in template | Pulumi dependency graph |
| **Secrets** | K8s Secrets | Pulumi secrets (encrypted in state) |
| **Observability** | Standard controller metrics | Prometheus metrics (5 metrics + histogram) |
| **GitOps Compatibility** | Native K8s | Server-Side Apply with field ownership |

## Deletion Logic

The Template controller implements robust deletion handling with proper dependency ordering and cleanup.

### Resource Dependency Graph

```
                     Template
                        │
          ┌─────────────┼─────────────┐
          │             │             │
          ▼             ▼             ▼
        CRD         Informer      Programs
          │                           │
          ▼                           │
      Instances ◄─────────────────────┘
          │
          ▼
       Stacks
          │
          ▼
  Cloud Resources (via Pulumi)
```

### Instance Deletion Flow

When an instance of a generated CRD is deleted, the controller handles cleanup based on the `destroyOnDelete` lifecycle setting.

#### With `destroyOnDelete: true` (Default)

```
Instance Delete Event
        │
        ▼
handleInstanceDeletion()
        │
        ▼
Check: Stack exists?
        │
        ├── Yes ──► Delete Stack (triggers Pulumi destroy)
        │                    │
        │                    ▼
        │           Wait for Stack destroy to complete
        │                    │
        │           Stack watch detects deletion
        │                    │
        │                    ▼
        │           cleanupInstanceFinalizers()
        │                    │
        │                    ▼
        │           Delete orphaned Program
        │           Remove instance finalizer
        │
        └── No (already gone) ──► Delete Program
                                  Remove instance finalizer
```

**Key behaviors:**
1. Deleting the Stack triggers Pulumi's destroy operation
2. The Stack's finalizer ensures cloud resources are destroyed
3. The Stack watch (`stackToTemplateMapper`) detects Stack deletion and enqueues Template reconciliation
4. `cleanupInstanceFinalizers()` removes finalizers from instances whose Stacks are gone

#### With `destroyOnDelete: false`

```
Instance Delete Event
        │
        ▼
handleInstanceDeletion()
        │
        ▼
Check: Stack exists?
        │
        ├── Yes ──► Patch Stack: DestroyOnFinalize = false
        │           Delete Stack (no Pulumi destroy)
        │           Delete Program
        │           Remove instance finalizer
        │
        └── No ──► Delete Program
                   Remove instance finalizer
```

**Key behaviors:**
1. Uses atomic `Patch` (not Update) to set `DestroyOnFinalize = false` before deletion
2. This prevents race conditions where the Stack controller might process deletion before the update
3. Cloud resources remain intact - useful for preserving data or migrating ownership

### Template Deletion Flow

When a Template is deleted, all associated resources must be cleaned up in the correct order.

```
Template Delete Event
        │
        ▼
handleDeletion()
        │
        ▼
listPendingStacks()
        │
        ├── Stacks exist? ── Yes ──► Requeue (5s)
        │                            (wait for all destroys to complete)
        │
        └── All Stacks gone? ──┐
                               │
                               ▼
                      deleteAssociatedPrograms()
                               │
                               ▼
                      cleanupInstanceFinalizers()
                               │
                               ▼
                      cleanupInformer()
                               │
                               ▼
                      deleteCRD()
                               │
                               ▼
                      Remove Template finalizer
```

**Key behaviors:**
1. **Wait for Stacks**: Template deletion waits for ALL Stacks to complete destroy operations
2. **Dependency ordering**: Stacks → Programs → CRD to prevent orphaned resources
3. **Finalizer cleanup**: Instance finalizers are removed BEFORE deleting the CRD
4. **CRD deletion**: Cascades to delete all instances (which are now finalizer-free)

### Stack Watch Pattern

The controller watches Stack deletions to trigger instance cleanup:

```go
Watches(
    &pulumiv1.Stack{},
    handler.EnqueueRequestsFromMapFunc(r.stackToTemplateMapper),
    builder.WithPredicates(predicate.Funcs{
        CreateFunc:  func(e event.CreateEvent) bool { return false },
        UpdateFunc:  func(e event.UpdateEvent) bool { return false },
        DeleteFunc:  func(e event.DeleteEvent) bool { return true },
        GenericFunc: func(e event.GenericEvent) bool { return false },
    }),
),
```

**Purpose:**
- When a Stack completes destroy and is deleted, the watch triggers Template reconciliation
- The reconciler then runs `cleanupInstanceFinalizers()` to unblock garbage collection
- Only delete events trigger reconciliation (create/update are filtered out)

### Error Handling

The deletion logic includes robust error handling:

1. **Atomic Stack updates**: Uses `Patch` instead of `Update` to prevent race conditions
2. **Error propagation**: `removeInstanceFinalizer` returns errors that are logged with context
3. **Retry on failure**: Errors don't block the reconciliation loop; next events trigger retries
4. **Graceful degradation**: Cleanup continues even if individual operations fail
