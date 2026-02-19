# Release

## Release Preparation

### Version tags

There's a Make target that updates some of the versions throughout the repository.

```bash
# Update version strings across the codebase
make prep RELEASE=v2.3.0
````

The tooling is not comprehensive, so manually verify the following files have been updated:

- README.md
- agent/version/version.go
- deploy/deploy-operator-yaml/Pulumi.yaml
- deploy/helm/pulumi-operator/Chart.yaml
- deploy/helm/pulumi-operator/README.md
- deploy/quickstart/install.yaml
- operator/Makefile
- operator/config/manager/kustomization.yaml
- operator/version/version.go

### Changelog

Changelog updates are entirely manual. Move entries from `Unreleased` to a new version section like so:

```shell
CHANGELOG
=========

## Unreleased

## 2.4.1 (2026-02-02)

- Move Unreleased items here
```

### Prepare release

Open a PR. [Here is an example.](https://github.com/pulumi/pulumi-kubernetes-operator/pull/1043)
Once merged, tag the head and push the tag to start the release workflow (.github/workflows/release.yaml).
You can use @release-bot for this internally.
The release workflow will create a GitHub release with a draft of the release notes.

### GitHub release

Verify that the GitHub release page properly shows the release changes. Use the UI to make any necessary adjustments.