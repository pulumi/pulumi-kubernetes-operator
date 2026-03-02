<!--
SYNC IMPACT REPORT
==================
Version change: [TEMPLATE] → 1.0.0 (initial ratification)

Modified principles:
  - All principles: NEW (filled from blank template)

Added sections:
  - Core Principles (5 principles)
  - Security & Compliance Requirements
  - Development Workflow & Review Process
  - Governance

Removed sections:
  - N/A (initial fill)

Templates requiring updates:
  - ✅ .specify/memory/constitution.md (this file)
  - ⚠ .specify/templates/plan-template.md — Constitution Check section uses generic gates;
        should reference project-specific checks (golangci-lint, envtest, CHANGELOG, CRD codegen).
  - ⚠ .specify/templates/spec-template.md — Success Criteria section should include
        performance/reliability SC examples aligned with Principle IV.
  - ⚠ .specify/templates/tasks-template.md — Polish phase should mark CHANGELOG update
        as MANDATORY per Principle V, not optional.
  - ⚠ .specify/templates/agent-file-template.md — Active Technologies and Commands
        sections should be seeded with Go/gRPC/Kubernetes project context.

Follow-up TODOs:
  - None: all fields resolved from codebase analysis.
-->

# Pulumi Kubernetes Operator Constitution

## Core Principles

### I. Code Quality & Maintainability (NON-NEGOTIABLE)

All Go source code MUST pass `golangci-lint` using the project's root `.golangci.yml`
configuration before any PR is merged. The enabled linter set is authoritative:
`depguard`, `goconst`, `gofmt`, `staticcheck`, `gosimple`, `gosec`, `govet`
(with `nilness`, `reflectvaluecompare`, `sortslice`, `unusedwrite` checks),
`ineffassign`, `misspell`, `nakedret`, `nolintlint`, and `unconvert`.

Every source file MUST carry the Apache License 2.0 header enforced by `goheader`.
The `github.com/golang/protobuf` package MUST NOT be imported; use
`google.golang.org/protobuf` instead (enforced by `depguard`).

`go fmt ./...` and `go vet ./...` MUST be run and pass as part of `make build`
before any commit. Naked returns are forbidden. `nolintlint` directives must
not be left unused.

Comments MUST only be added for non-obvious logic or exported/public APIs.
Self-documenting code with clear names is preferred over explanatory comments.
Debug statements, commented-out code, and unused imports MUST be removed before
any PR is opened.

**Rationale**: This project has multiple active contributors and production
deployments. Consistent style and static analysis catch real bugs (nil
dereferences, reflect misuse, security issues) before they reach users.

### II. Testing Standards (NON-NEGOTIABLE)

`make test` MUST pass locally before a PR is opened. The test suite includes:

- **Unit/controller tests**: Run via `envtest` with Kubernetes API server assets
  (`ENVTEST_K8S_VERSION = 1.32.0`). MUST cover new controllers, reconciler
  logic, webhook validators, and error paths.
- **E2E tests**: MUST be run against a Kind cluster (`make test-e2e`) for any
  change affecting the full Stack→Workspace→Update→Agent workflow. The timeout
  is 60 minutes.
- **Coverage**: Tracked via Codecov. PRs MUST NOT decrease overall project
  coverage. New features MUST include tests sufficient to maintain or improve
  coverage.

External contributor PRs require a maintainer to explicitly invoke
`/run-acceptance-tests` before merge. This gate MUST NOT be bypassed.

Tests MUST be written before or alongside implementation — never after a PR is
already open without tests. TDD is strongly encouraged for new controllers and
agent commands.

**Rationale**: The operator manages real cloud infrastructure. A failing
reconciler can leave stacks in broken states. Test coverage is the primary
defense against regressions in production deployments.

### III. User Experience Consistency

The operator's API surface MUST follow Kubernetes API conventions throughout:

- **CRD API design**: Field names, types, and defaults MUST follow the
  Kubernetes API conventions. Status conditions MUST use the standard
  `Ready`, `Reconciling`, and `Stalled` condition types.
- **Error surfacing**: Errors MUST be propagated to the relevant CR's
  `.status.conditions` with actionable messages. Vague messages like
  "error occurred" are forbidden; messages MUST include the resource name,
  operation attempted, and root cause.
- **Helm chart**: MUST lint cleanly with `ct lint` (chart-testing) and
  `ah lint` (Artifact Hub). Helm values changes MUST maintain backward
  compatibility or provide a documented migration path.
- **Breaking API changes**: Any change removing or renaming a CRD field,
  changing a field type, or altering validation rules MUST increment the
  API group version and provide a conversion webhook or migration guide.
- **Printer columns**: `kubectl get` output MUST include meaningful status
  columns (e.g., Ready, State, Last Update) so operators can assess health
  without `kubectl describe`.

**Rationale**: Kubernetes operators are infrastructure primitives. Users rely
on consistent, predictable API behavior across upgrades. Inconsistent status
conditions or breaking changes without migration paths cause production
incidents.

### IV. Performance & Reliability Requirements

The operator MUST remain responsive under load and in degraded conditions:

- **Non-blocking reconcilers**: Controller reconcile loops MUST NOT perform
  blocking I/O or long-running Pulumi operations inline. All Pulumi executions
  MUST be delegated to the agent via gRPC.
- **Connection management**: gRPC connections to workspace agent pods MUST be
  managed via the `ConnectionManager` (cached, not created per-reconcile).
  New connection patterns MUST reuse this mechanism.
- **Concurrency**: Default maximum concurrent reconciles is 25 and MUST remain
  configurable via operator flags. Controllers MUST use generation-based
  predicates to avoid unnecessary reconcile thrashing.
- **Leader election**: HA mode with leader election MUST remain functional.
  Configurable lease duration and renew deadlines MUST be preserved to avoid
  interrupting long-running Pulumi operations.
- **Garbage collection**: Update objects MUST be garbage-collected per the
  TTL and garbage-collection policy. Workspace pods MUST be reclaimable per
  `WorkspaceReclaimPolicy`. Memory leaks via unbounded object accumulation
  are a P0 defect.
- **Workspace stability**: Changes to workspace StatefulSet specs MUST be
  idempotent. Non-deterministic ordering of init containers or config MUST be
  avoided to prevent unnecessary pod restarts.

**Rationale**: The operator manages infrastructure deployments that can take
minutes. Blocking the reconcile loop or creating excessive pod churn under load
directly impacts user SLAs and cloud resource costs.

### V. Documentation Standards

Documentation MUST be treated as a first-class deliverable, not an afterthought:

- **CHANGELOG.md**: MUST be updated with every user-facing change (features,
  bug fixes, breaking changes, deprecations) before the PR is merged. Each
  entry MUST link to the relevant PR or issue. The format follows the project's
  established convention: `- Description of change [#NNN](link)`.
- **CRD API reference**: Auto-generated via `make generate-crdocs`. MUST be
  regenerated whenever CRD schemas change. The generated docs in `docs/` MUST
  be committed alongside schema changes in the same PR.
- **Helm chart README**: MUST stay in sync with `Chart.yaml` changes and new
  configurable values. Use `helm-docs` or equivalent when chart values change.
- **PR descriptions**: MUST include a "Proposed changes" section explaining
  what the change does and why. References to related issues MUST be included
  using `Fixes #NNN` or `Closes #NNN` syntax where applicable.
- **Release notes**: Prepared via `make prep RELEASE=vX.Y.Z` which updates
  version strings across the codebase. Release notes MUST be drafted from the
  CHANGELOG before tagging.

**Rationale**: This project has multiple releases and external contributors.
Undocumented changes create support burden and erode trust. The CHANGELOG is
the primary artifact users rely on to assess upgrade safety.

## Security & Compliance Requirements

Security is a non-negotiable property of this production operator:

- **Static analysis**: `gosec` linter is REQUIRED and MUST pass on all code.
  Security findings of HIGH or CRITICAL severity are blocking.
- **Container image scanning**: Trivy IaC scanning MUST run on Helm chart
  changes (CI-enforced). CRITICAL and HIGH severity findings MUST be
  resolved or explicitly suppressed with documented justification.
- **Dependency security**: Renovate manages dependency updates. Security PRs
  from Renovate (tagged `[SECURITY]`) MUST be reviewed and merged promptly —
  target within 5 business days of creation.
- **Secret hygiene**: No secrets, tokens, API keys, or credentials MUST be
  committed to the repository. The agent uses Kubernetes `TokenReview`-based
  authentication; alternative auth mechanisms MUST follow the same pattern.
- **License headers**: All source files MUST carry the Apache License 2.0
  header (enforced by `goheader` linter). Third-party dependencies MUST be
  license-compatible with Apache 2.0.
- **Minimal permissions**: Kubernetes RBAC for the operator ServiceAccount MUST
  follow the principle of least privilege. New permissions MUST be justified in
  the PR description.

## Development Workflow & Review Process

### Pull Request Standards

- Every PR MUST pass the full CI pipeline (build, lint, unit tests) before
  review. Merging with failing checks is forbidden.
- External contributor PRs MUST have a maintainer run `/run-acceptance-tests`
  before merge. Build and lint alone are insufficient for external changes.
- PRs MUST reference the related issue using `Fixes #NNN` or `Closes #NNN`
  in the description.
- CRD schema changes MUST include regenerated manifests (`make generate-crds`)
  and regenerated docs (`make generate-crdocs`) in the same PR.
- Dependency changes MUST be validated with `go mod verify` and MUST not
  introduce `replace` directives without explicit justification.

### Versioning & Release Policy

- The project follows semantic versioning: `vMAJOR.MINOR.PATCH`.
  - **MAJOR**: Breaking CRD API changes, removed features, or incompatible
    behavioral changes requiring user migration.
  - **MINOR**: New features, new CRD fields (backward-compatible), new
    configuration options.
  - **PATCH**: Bug fixes, security patches, dependency updates, documentation
    corrections.
- Release preparation MUST use `make prep RELEASE=vX.Y.Z` to update all
  version strings consistently.
- Releases are tagged on `master` after the release PR is merged. The release
  workflow creates a GitHub Release automatically.

### Code Generation

- Controller-generated code (DeepCopy, apply configurations, CRD manifests)
  MUST be regenerated via `make generate` / `make manifests` whenever API
  types change.
- Protobuf/gRPC code MUST be regenerated via `cd agent && make protoc` when
  `.proto` files change.
- Generated files MUST NOT be manually edited.

## Governance

This constitution supersedes all informal conventions, verbal agreements, and
prior practice. When a discrepancy exists between this document and other
project files, this constitution takes precedence.

**Amendment procedure**:
1. Open a PR modifying `.specify/memory/constitution.md`.
2. Update `CONSTITUTION_VERSION` following semantic versioning rules.
3. Update `LAST_AMENDED_DATE` to the amendment date.
4. Propagate changes to dependent templates (plan, spec, tasks) in the same PR.
5. The PR requires approval from at least one project maintainer.
6. Document the amendment rationale in the CHANGELOG under `## Unreleased`.

**Compliance review**: Verified at every major and minor release. The release
PR checklist MUST confirm compliance with all five core principles.

**Complexity justification**: Any deviation from these principles (e.g., a
blocking reconcile for a narrowly-scoped operation) MUST be documented in the
PR with explicit rationale and tracked in the Complexity Tracking table of the
feature's `plan.md`.

For runtime development guidance, see `CLAUDE.md` at the project root and the
operator-specific `operator/CLAUDE.md`.

**Version**: 1.0.0 | **Ratified**: 2026-03-02 | **Last Amended**: 2026-03-02
