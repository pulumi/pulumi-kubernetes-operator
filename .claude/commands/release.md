---
description: Prepare a new release
---

Prepare a new release of the pulumi-kubernetes-operator by:

1. Ask the user for the release version (e.g., v2.3.0)
2. Run `make prep RELEASE=<version>` to update version strings across the codebase
3. Review the changes made by the prep command
4. **IMPORTANT: Manual Helm Chart Updates** - The `make prep` command does NOT update these fields:
   - Update the `version` field in `deploy/helm/pulumi-operator/Chart.yaml` (e.g., from "2.2.0" to "2.3.0" - note: no 'v' prefix)
   - Update the version badge in `deploy/helm/pulumi-operator/README.md` line 3 (e.g., `![Version: 2.2.0]` to `![Version: 2.3.0]`)
5. Update CHANGELOG.md by moving unreleased items to a new release section
6. Create a commit with message "Prepare release <version>"
7. Guide the user to:
   - Open a PR with the changes
   - After the PR is merged, tag the release with `git tag <version>` and `git push origin <version>`
   - This will trigger the release workflow (.github/workflows/release.yaml)
   - The workflow will create a GitHub release with a draft of the release notes
8. Once the draft release is created, fetch the list of merged PRs since the last release
9. Generate a "What's New" section summarizing the key changes from the PRs
10. Guide the user to update the GitHub release notes with the "What's New" section

Ask the user for the version number now.
