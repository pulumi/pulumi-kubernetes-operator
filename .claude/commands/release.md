---
description: Prepare a new release
---

Prepare a new release of the pulumi-kubernetes-operator by:

1. Ask the user for the release version (e.g., v2.3.0)
2. Run `make prep RELEASE=<version>` to update version strings across the codebase
3. Review the changes made by the prep command
4. Create a commit with message "Prepare release <version>"
5. Guide the user to:
   - Open a PR with the changes
   - After the PR is merged, tag the release with `git tag <version>` and `git push origin <version>`
   - This will trigger the release workflow (.github/workflows/release.yaml)
   - The workflow will create a GitHub release with a draft of the release notes
6. Once the draft release is created, fetch the list of merged PRs since the last release
7. Generate a "What's New" section summarizing the key changes from the PRs
8. Guide the user to update the GitHub release notes with the "What's New" section

Ask the user for the version number now.
