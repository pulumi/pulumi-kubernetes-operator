CHANGELOG
=========

## HEAD (Unreleased)
(None)

---

## 0.0.11 (2021-04-29)
- Bump to v3.1.0 and GA automation api [#137](https://github.com/pulumi/pulumi-kubernetes-operator/pull/137)

- Bug fix for secret manager switching to pulumi-console on updates [#137](https://github.com/pulumi/pulumi-kubernetes-operator/pull/137)

- INFO logging level by default [#138](https://github.com/pulumi/pulumi-kubernetes-operator/pull/138)

- Allow Go applications to build [#141](https://github.com/pulumi/pulumi-kubernetes-operator/pull/141)

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
