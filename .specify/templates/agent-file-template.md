# Pulumi Kubernetes Operator Development Guidelines

Auto-generated from all feature plans. Last updated: [DATE]

## Active Technologies

- **Language**: Go 1.24+
- **Framework**: controller-runtime (kubebuilder v4.2.0)
- **Agent communication**: gRPC + Protobuf
- **Infrastructure API**: Pulumi Automation API
- **Packaging**: Helm chart + Kustomize
- **Testing**: envtest (unit/controller), Kind (e2e)
- **Linting**: golangci-lint v1.64.2 with `.golangci.yml`

[EXTRACTED FROM ALL PLAN.MD FILES]

## Project Structure

```text
operator/          # Control plane (controllers, webhooks, CRD types)
agent/             # gRPC server executing Pulumi operations
deploy/            # Helm chart, Kustomize manifests, CRDs
docs/              # Auto-generated CRD API reference
```

[ACTUAL STRUCTURE FROM PLANS]

## Commands

```bash
make build          # Build agent + operator
make test           # Run all tests (envtest)
make lint           # golangci-lint
make codegen        # Regenerate CRDs + docs
cd operator && make test-e2e  # E2e tests (requires Kind)
make prep RELEASE=vX.Y.Z      # Prepare release
```

[ONLY COMMANDS FOR ACTIVE TECHNOLOGIES]

## Code Style

- All files: Apache 2.0 license header (enforced by goheader linter)
- No `github.com/golang/protobuf` — use `google.golang.org/protobuf`
- `go fmt ./...` and `go vet ./...` must pass
- Generated files MUST NOT be manually edited

[LANGUAGE-SPECIFIC, ONLY FOR LANGUAGES IN USE]

## Recent Changes

[LAST 3 FEATURES AND WHAT THEY ADDED]

<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->
