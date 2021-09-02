CHANGELOG
=========

## HEAD (Unreleased)
(None)

## 0.0.19 (2021-09-02)
- Add support for safe upgrades and graceful shutdowns [#189](https://github.com/pulumi/pulumi-kubernetes-operator/pull/189)
- Bump pulumi dependencies [#189](https://github.com/pulumi/pulumi-kubernetes-operator/pull/189)

## 0.0.18 (2021-08-31)
- Fix pip cache directory to be on emptyDir mount [#181](https://github.com/pulumi/pulumi-kubernetes-operator/pull/181)
- Add `useLocalStackOnly` option to prevent stack creation by the operator [#186](https://github.com/pulumi/pulumi-kubernetes-operator/pull/186)
- Fix config loading [#187](https://github.com/pulumi/pulumi-kubernetes-operator/pull/187)

## 0.0.17 (2021-08-18)

- Bump controller-runtime to support graceful shutdown/upgrades [#178](https://github.com/pulumi/pulumi-kubernetes-operator/pull/178)
- Update to v3.10.2 [#177](https://github.com/pulumi/pulumi-kubernetes-operator/pull/177)
- Cloak outputs with secrets in stack CR [#177](https://github.com/pulumi/pulumi-kubernetes-operator/pull/177)

## 0.0.16 (2021-07-29)

- Ensure either 'branch' or 'commit' is set in stack CR & bump pulumi/pulumi to 3.9.0 [#168](https://github.com/pulumi/pulumi-kubernetes-operator/pull/168)

## 0.0.15 (2021-07-23)

- Automatically track git branches without a specified commit.
  If a branch is specified, the operator will poll the repo every minute and automatically deploy
  new commits to the branch.
  [#162](https://github.com/pulumi/pulumi-kubernetes-operator/pull/162)

## 0.0.14 (2021-07-01)
- Update deployment manifests & code for pulumi v3.6.0 [#159](https://github.com/pulumi/pulumi-kubernetes-operator/pull/159)

## 0.0.13 (2021-05-25)
- Bump pulumi/pulumi to v3.3.1 and add user-agent string for automation-api [#156](https://github.com/pulumi/pulumi-kubernetes-operator/pull/156)

## 0.0.12 (2021-05-21)
- Bump to v3.3.0 [#152](https://github.com/pulumi/pulumi-kubernetes-operator/pull/152)
- Correctly handle repoDir [#151](https://github.com/pulumi/pulumi-kubernetes-operator/pull/151)

## 0.0.11 (2021-04-29)
- Bump to v3.1.0 and GA automation api [#137](https://github.com/pulumi/pulumi-kubernetes-operator/pull/137)

- Bug fix for secret manager switching to pulumi-console on updates [#137](https://github.com/pulumi/pulumi-kubernetes-operator/pull/137)

- INFO logging level by default [#138](https://github.com/pulumi/pulumi-kubernetes-operator/pull/138)

- Allow Go applications to build [#141](https://github.com/pulumi/pulumi-kubernetes-operator/pull/141)

- Update docs for pulumi v3 providers & misc [#136](https://github.com/pulumi/pulumi-kubernetes-operator/pull/136)

## 0.0.10 (2021-04-05)
- Bump to 2.23.2 and add SecretRefs to allow secrets to be specified through
  references [#130](https://github.com/pulumi/pulumi-kubernetes-operator/pull/130)

- Bump base image to 2.24.1 and added resource ref variant for GitAuthSecret
  [#132](https://github.com/pulumi/pulumi-kubernetes-operator/pull/132)

## 0.0.9 (2021-03-26)
- Fix integration tests. Bumps embedded pulumi to 2.17.0.
  [#115](https://github.com/pulumi/pulumi-kubernetes-operator/pull/115)

- Make environment variable population more generic and bump base image to 2.23.1
  [#125](https://github.com/pulumi/pulumi-kubernetes-operator/pull/125)

- Regenerate CRD for apiextensions/v1 (v1beta1 deprecated) <br/>
  **BREAKING** Your Kubernetes cluster must now be v1.16 or higher
  [#127](https://github.com/pulumi/pulumi-kubernetes-operator/pull/127)

## 0.0.8 (2020-12-03)

- Use ephemeral storage for disk mutations.
  [#109](https://github.com/pulumi/pulumi-kubernetes-operator/pull/109)

- Fix handling of `state` message, and add new `lastAttemptedCommit` and `lastSuccessfulCommit` status fields.
  [#107](https://github.com/pulumi/pulumi-kubernetes-operator/pull/107).

- Support configuring alternative `backend`s.
  [#106](https://github.com/pulumi/pulumi-kubernetes-operator/pull/106)

- Support streaming logs from `pulumi up/destroy/refresh`.
  [#101](https://github.com/pulumi/pulumi-kubernetes-operator/pull/101)

## 0.0.7 (2020-09-29)

- Add SSH keyagent, add keys to known_hosts for SSH git.
  [#92](https://github.com/pulumi/pulumi-kubernetes-operator/pull/92)

## 0.0.6 (2020-09-15)

- Refactor controller to use Automation API.
  [#86](https://github.com/pulumi/pulumi-kubernetes-operator/pull/86)

## 0.0.5 (2020-08-11)

- Initial Release!
