---
permissions:
  contents: read
  pull-requests: read
  id-token: write
engine:
  id: claude
  env:
    ANTHROPIC_API_KEY: ${{ steps.esc-secrets.outputs.ANTHROPIC_API_KEY || '__GH_AW_ACTIVATION_PLACEHOLDER__' }}
steps:
  - env:
      ESC_ACTION_ENVIRONMENT: github-secrets/pulumi-labs-gh-aw-internal
      ESC_ACTION_EXPORT_ENVIRONMENT_VARIABLES: "false"
      ESC_ACTION_OIDC_AUTH: "true"
      ESC_ACTION_OIDC_ORGANIZATION: pulumi
      ESC_ACTION_OIDC_REQUESTED_TOKEN_TYPE: urn:pulumi:token-type:access_token:organization
    id: esc-secrets
    name: Fetch secrets from ESC
    uses: pulumi/esc-action@6cf9520e68354d86f81c455e8d43eabd58f5c9f5  # v1.5.0
  - name: Validate ESC secret output
    env:
      ANTHROPIC_API_KEY_FROM_ESC: ${{ steps.esc-secrets.outputs.ANTHROPIC_API_KEY }}
    run: |
      test -n "$ANTHROPIC_API_KEY_FROM_ESC" || {
        echo "ESC did not return ANTHROPIC_API_KEY";
        exit 1;
      }
tools:
  cache-memory: true
  github:
    toolsets: [pull_requests, repos]
safe-outputs:
  threat-detection: false
  create-pull-request-review-comment:
    max: 12
    side: "RIGHT"
    target: "${{ github.event.pull_request.number || github.event.inputs.pr_number || github.event.issue.number }}"
    target-repo: "${{ github.repository }}"
  resolve-pull-request-review-thread:
    max: 12
    target: "${{ github.event.pull_request.number || github.event.inputs.pr_number || github.event.issue.number }}"
    target-repo: "${{ github.repository }}"
  submit-pull-request-review:
    max: 1
    target: "${{ github.event.pull_request.number || github.event.inputs.pr_number || github.event.issue.number }}"
  noop:
    max: 1
  messages:
    footer: "> Reviewed by [{workflow_name}]({run_url})"
    run-started: "Started automated PR review for #${{ github.event.pull_request.number || github.event.inputs.pr_number || github.event.issue.number }}."
    run-success: "Finished automated PR review for #${{ github.event.pull_request.number || github.event.inputs.pr_number || github.event.issue.number }}."
    run-failure: "Automated PR review failed for #${{ github.event.pull_request.number || github.event.inputs.pr_number || github.event.issue.number }} ({status})."
---


Review pull request #${{ github.event.pull_request.number || github.event.inputs.pr_number || github.event.issue.number }} in repository `${{ github.repository }}`.

Workflow-specific rules:
- Use `${{ github.event.pull_request.number || github.event.inputs.pr_number || github.event.issue.number }}` as the authoritative PR target.
- Treat the imported review prompt as the source of truth for review procedure, finding severity, safe-output usage, cache-memory behavior, and final review event selection.
- Use only the configured gh-aw safe-output tools for review side effects; do not use free-form issue comments or other side channels.
- If the PR is not reviewable or required context is missing, call `noop`.
- Ignore discovery steps intended for runs without PR context.
