# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

The Pulumi Kubernetes Operator is a Kubernetes operator that provides a CI/CD workflow for Pulumi stacks using Kubernetes primitives. It allows users to adopt a GitOps workflow for managing cloud infrastructure by creating Stack resources as first-class Kubernetes API resources.

## Architecture

The project consists of two main executable components:

### Operator Manager (`operator/`)
- The main control plane component running in the cluster
- Uses Kubernetes controller-runtime framework
- Implements 4 primary controllers:
  - **StackReconciler** - Manages Stack CRs (pulumi.com/v1 and v1alpha1)
  - **WorkspaceReconciler** - Manages Workspace CRs (auto.pulumi.com/v1alpha1)
  - **UpdateReconciler** - Manages Update CRs (auto.pulumi.com/v1alpha1)
  - **ProgramReconciler** - Manages Program CRs (pulumi.com/v1)

### Agent (`agent/`)
- A gRPC service running inside workspace pods
- Executes actual Pulumi operations (up, destroy, preview, refresh)
- Uses Pulumi Automation API to interact with Pulumi stacks
- Communicates with the operator via gRPC with Kubernetes TokenReview-based authentication
- Provides an "init" command for workspace initialization

### Key Workflow
1. User creates a **Stack CR** with source (Git URL, Flux source, or Program ref)
2. **StackReconciler** validates spec, resolves sources, creates **Workspace CR**
3. **WorkspaceReconciler** creates StatefulSet + Service, bootstraps workspace pod with program source
4. **StackReconciler** creates an **Update CR** (type=up/destroy/preview/refresh)
5. **UpdateReconciler** connects to workspace pod via gRPC and streams results from agent
6. **Agent** executes Pulumi operation using Automation API and returns outputs

## Common Commands

### Building
```bash
# Build both agent and operator binaries (fast, dynamically-linked)
make build

# Build just the operator
make build-operator
# or: cd operator && make build

# Build just the agent
make build-agent
# or: cd agent && make build

# Build operator Docker image (statically-linked)
make build-image
# or: cd operator && make docker-build

# Build quickstart install manifest
make build-deploy
```

### Testing
```bash
# Run all tests (operator + agent)
make test

# Run operator tests only
cd operator && make test

# Run agent tests only
cd agent && make test

# Run e2e tests (requires Kind cluster)
cd operator && make test-e2e
```

### Linting
```bash
# Lint all code
make lint

# Auto-fix linting issues
cd operator && make lint-fix
cd agent && make lint-fix
```

### Code Generation
```bash
# Generate CRDs and documentation
make codegen

# Generate CRDs only
make generate-crds
# This runs: cd operator && make manifests
# Then copies CRDs to deploy/crds/ and deploy/helm/pulumi-operator/crds/

# Generate CRD documentation
make generate-crdocs

# Generate deep copy methods and apply configurations
cd operator && make generate
```

### Protocol Buffers
```bash
# Regenerate gRPC code from .proto files
cd agent && make protoc
```

### Deployment
```bash
# Install CRDs into current cluster
make install-crds
# or: cd operator && make install

# Deploy operator to current cluster
make deploy
# or: cd operator && make deploy
```

### Local Development
```bash
# Run operator locally (not in cluster)
cd operator && make run
# This sets WORKSPACE_LOCALHOST=localhost:50051 for local agent connection
```

## Repository Structure

```
pulumi-kubernetes-operator/
├── operator/                   # Operator code
│   ├── api/                   # CRD type definitions
│   │   ├── pulumi/v1/        # Stack, Program CRDs (current)
│   │   ├── pulumi/v1alpha1/  # Stack CRD (legacy)
│   │   └── auto/v1alpha1/    # Workspace, Update CRDs
│   ├── internal/
│   │   ├── controller/       # Reconciliation logic
│   │   │   ├── pulumi/       # Stack & Program controllers
│   │   │   └── auto/         # Workspace & Update controllers
│   │   ├── webhook/          # Validating webhooks
│   │   └── apply/            # Generated apply configurations
│   ├── cmd/main.go           # Operator entry point
│   └── config/               # Kustomize deployment configs
├── agent/                     # Agent code
│   ├── cmd/serve.go          # Agent gRPC server setup
│   ├── pkg/
│   │   ├── server/           # gRPC server implementation
│   │   ├── proto/            # Protobuf definitions & generated code
│   │   └── client/           # Credential helpers
│   └── main.go
├── deploy/                    # Deployment manifests
│   ├── crds/                 # Generated CRD manifests
│   ├── helm/                 # Helm chart
│   ├── quickstart/           # Quick install YAML
│   └── deploy-operator-yaml/ # Pulumi deployment program
├── examples/                  # Example Stack CRs
└── docs/                      # Documentation
```

## Key Technologies

- **controller-runtime**: Kubernetes controller framework
- **Pulumi Automation API**: Programmatic Pulumi interaction
- **gRPC + Protobuf**: Agent communication protocol
- **Flux CD API**: Integration with Flux sources for GitOps
- **go-git**: Git repository operations
- **Helm**: Configuration management for deployment (option 1)
- **Kustomize**: Configuration management for deployment (option 2)
- **kubebuilder**: Controller scaffolding (v4.2.0)

## CRD Relationships

- **Stack** → creates **Workspace** → creates StatefulSet + Service
- **Stack** → creates **Update** → connects to Workspace pod → executes via Agent
- **Stack** can reference ConfigMap and Secret for configuration
- **Stack** can reference **Program** for inline YAML programs
- **Update** → creates Secret with stack outputs
- **Stack** can specify `.spec.prerequisites` referencing other Stacks for dependency ordering
- **Stack** requires a ServiceAccount with ClusterRoleBinding for workspace pod

## Important Patterns

### Controller Patterns
- Owner references: Workspace owns StatefulSet; Update owns outputs Secret
- Finalizers: Clean resource deletion (destroy resources when Stack is deleted)
- Field indexing: Fast lookups for Stack→Program and Stack→Prerequisites relationships
- Predicates: Generation-based change detection
- Status conditions: Standard Kubernetes conditions (Ready, Reconciling, Stalled)

### Concurrency
- Max concurrent reconciles: Configurable (default 25)
- ConnectionManager: Caches gRPC connections to workspace pods
- Leader election: Optional HA mode for multiple operator replicas

### Source Resolution
- **GitSource**: SSH/HTTPS clone via go-git in agent bootstrap init container
- **FluxSource**: HTTP artifact download from Flux Source Controller
- **ProgramRef**: HTTP from program file server
- **LocalSource**: Filesystem volume mount (Kubernetes volume source) or customized workspace image

## Release Preparation

```bash
# Update version strings across the codebase
make prep RELEASE=v2.3.0

# This updates:
# - README.md
# - agent/version/version.go
# - operator/Makefile
# - operator/version/version.go
# - deploy/quickstart/install.yaml
# - deploy/deploy-operator-yaml/Pulumi.yaml
# - deploy/helm/pulumi-operator/Chart.yaml
```

- Update the changelog by moving the unreleased items to a new section for the release.
- Prepare the code, commit and open a PR.
- Once merged, tag the head and push the tag to start the release workflow (.github/workflows/release.yaml).
- The release workflow will create a GitHub release with a draft of the release notes.
 
## Development Tips

- Before running a command, be aware of the current directory. 

### Running Single Tests
```bash
# Operator tests
cd operator && go test -v -run TestStackReconciler ./internal/controller/pulumi/

# Agent tests
cd agent && go test -v -run TestServerUp ./pkg/server/
```

### Viewing CRD Schema
```bash
# CRD manifests are in deploy/crds/
cat deploy/crds/pulumi.com_stacks.yaml
cat deploy/crds/auto.pulumi.com_workspaces.yaml
cat deploy/crds/auto.pulumi.com_updates.yaml
cat deploy/crds/pulumi.com_programs.yaml
```

### Debugging Controllers Locally
The operator can run locally against a remote cluster for faster iteration:
```bash
cd operator && make run
# Sets WORKSPACE_LOCALHOST=localhost:50051 to connect to local (or port-forwarded) agent 
# Sets SOURCE_CONTROLLER_LOCALHOST=localhost:9090 for local (or port-forwarded) Flux source controller
```

To port-forward to the agent in a workspace pod, you would use:

```bash
kubectl port-forward -n <namespace> <workspace-pod-name> 50051:50051
```

### Checking Operator Logs
```bash
kubectl logs -n pulumi-kubernetes-operator deployment/controller-manager -f
```

## Documentation Files

- `docs/build.md` - Build and development guide
- `docs/stacks.md` - Stack CR API reference (auto-generated)
- `docs/programs.md` - Program CR API reference (auto-generated)
- `docs/workspaces.md` - Workspace CR API reference (auto-generated)
- `docs/updates.md` - Update CR API reference (auto-generated)
- `docs/metrics.md` - Prometheus metrics integration guide
