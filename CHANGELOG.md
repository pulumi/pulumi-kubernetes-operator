CHANGELOG
=========

## Unreleased

- Improved support for using custom program sources. [#741](https://github.com/pulumi/pulumi-kubernetes-operator/pull/741) 
- Improved Status logging. [#742](https://github.com/pulumi/pulumi-kubernetes-operator/pull/742)

## 2.0.0-beta.1 (2024-10-18)

- Major update to new horizontally-scalable architecture. [#725](https://github.com/pulumi/pulumi-kubernetes-operator/pull/725)

## 1.16.0 (2024-08-26)

### Fixed
- Respect `serviceAccount.name` in Helm chart values. [#530](https://github.com/pulumi/pulumi-kubernetes-operator/pull/530)

### Changed
- Update pulumi/pulumi to v3.130.0. [#629](https://github.com/pulumi/pulumi-kubernetes-operator/pull/638)
- Regenerate CRDs with controller-gen v0.15.0. [#581](https://github.com/pulumi/pulumi-kubernetes-operator/pull/581)

## 1.15.0 (2024-04-12)
- Clean up stale workspace directories and don't treat them as a crude lock. [#552](https://github.com/pulumi/pulumi-kubernetes-operator/pull/552)
- Fixed `nodeSelector`, `affinity`, and `tolerations` Helm chart values that were previously effectively ignored.
  [#548](https://github.com/pulumi/pulumi-kubernetes-operator/pull/548)
- Update pulumi/pulumi to v3.92.0 [#559](https://github.com/pulumi/pulumi-kubernetes-operator/pull/559)

## 1.14.0 (2023-11-08)
- Changed indentation in deploy/helm/pulumi-operator/templates/deployment.yaml for volumes and volumeMounts.
- Use a separate PULUMI_HOME for each stack. [#490](https://github.com/pulumi/pulumi-kubernetes-operator/pull/490)
- Use Go v1.21 for builds. [#504](https://github.com/pulumi/pulumi-kubernetes-operator/pull/504)
- Update pulumi/pulumi to v3.109.0 [#517](https://github.com/pulumi/pulumi-kubernetes-operator/pull/517)

## 1.13.0 (2023-08-04)
- Use digest field for Flux source artifact if present [#459](https://github.com/pulumi/pulumi-kubernetes-operator/pull/459)
- Add Helm chart support for installing the Pulumi Operator. [#379](https://github.com/pulumi/pulumi-kubernetes-operator/pull/379)

## 1.12.1 (2023-05-19)
- Update to [Pulumi SDK v3.68.0](https://github.com/pulumi/pulumi/releases/tag/v3.68.0) and the base
  image of the same version.

## 1.12.0 (2023-05-09)
- Enable refreshing dynamic credentials in provider state using targeted
  stack refresh [#443](https://github.com/pulumi/pulumi-kubernetes-operator/pull/443)
- Update to [Pulumi SDK v3.66.0](https://github.com/pulumi/pulumi/releases/tag/v3.66.0) and the base
  image of the same version.
- Added lastUpdate state update to ensure that synchronized commits display a succeeded status.
  [#429](https://github.com/pulumi/pulumi-kubernetes-operator/pull/429)

## 1.11.5 (2023-04-27)
- Update to [Pulumi SDK v3.65.1](https://github.com/pulumi/pulumi/releases/tag/v3.65.1) and the base
  image of the same version, fixing [issue
  #437](https://github.com/pulumi/pulumi-kubernetes-operator/issues/437)

## 1.11.4 (2023-04-26)
- Update to [Pulumi SDK v3.65.0](https://github.com/pulumi/pulumi/releases/tag/v3.65.0) and the base
  image of the same version, fixing [issue
  #435](https://github.com/pulumi/pulumi-kubernetes-operator/issues/435)

## 1.11.3 (2023-04-19)

- Update to [Pulumi SDK v3.64.0](https://github.com/pulumi/pulumi/releases/tag/v3.64.0) and the base
  image of the same version, fixing [issue
  #432](https://github.com/pulumi/pulumi-kubernetes-operator/issues/432)

## 1.11.2 (2023-03-10)

- Update to [Pulumi SDK v3.57.1](https://github.com/pulumi/pulumi/releases/tag/v3.57.1) and the base
  image of the same version, fixing [issue
  #411](https://github.com/pulumi/pulumi-kubernetes-operator/issues/411)
  [#422](https://github.com/pulumi/pulumi-kubernetes-operator/pull/422)

## 1.11.1 (2023-02-08)

- Work in sympathy with the Go build cache, by using stable build paths
  [#407](https://github.com/pulumi/pulumi-kubernetes-operator/pull/407)
- Update to [Pulumi SDK v3.55.0](https://github.com/pulumi/pulumi/releases/tag/v3.55.0) and the base
  image of the same version
  [#412](https://github.com/pulumi/pulumi-kubernetes-operator/pull/412)

## 1.11.0 (2023-01-25)

- Expand the installation programs in deploy/ so they can deploy the operator to several namespaces
  in one go, as well as upgrade the operator version.
  [#328](https://github.com/pulumi/pulumi-kubernetes-operator/pull/328)
- Avoid some needless and misleading log messages
  [#363](https://github.com/pulumi/pulumi-kubernetes-operator/pull/363)
- Bump Pulumi SDK to v3.46.0
  [#365](https://github.com/pulumi/pulumi-kubernetes-operator/pull/365)
- Rewrite test case to confirm to Pulumi YAML 1.0 (breaking) changes
  [#369](https://github.com/pulumi/pulumi-kubernetes-operator/pull/369)
- Use an init process so processes spawned by `pulumi` are reaped
  [#367](https://github.com/pulumi/pulumi-kubernetes-operator/pull/367)
- When a Stack uses a Flux source, but the source has no artifact to download, park the Stack until
  the source has been updated, rather than retrying
  [#359](https://github.com/pulumi/pulumi-kubernetes-operator/pull/359)
- Correct the stacks_failing metric in the case of a stack being deleted after failing
  [#402](https://github.com/pulumi/pulumi-kubernetes-operator/pull/402)

## 1.10.1 (2022-10-25)

- Give an example of using this operator with a Flux GitRepository and webhooks, in
  `examples/flux-source`.
  [#339](https://github.com/pulumi/pulumi-kubernetes-operator/pull/339)
- De-escalate a log message about a harmless error from ERROR to DEBUG
  [#352](https://github.com/pulumi/pulumi-kubernetes-operator/pull/352)
- Watch source kinds and Programs to react to changes
  [#348](https://github.com/pulumi/pulumi-kubernetes-operator/pull/348)

## 1.10.0 (2022-10-21)

- Make .ContinueResyncOnCommitMatch apply to all sources (git, Flux sources, or Program objects)
  [#346](https://github.com/pulumi/pulumi-kubernetes-operator/pull/346)

## 1.10.0-rc.1 (2022-10-18) (release candidate)

- Make `.spec.projectRepo` optional in the Stack CRD. This is technically a breaking change
  according to the advice in [Kubernetes API
  guidelines](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api_changes.md#on-compatibility),
  but is not expected to affect any deployment. In particular, all Stack objects which are valid now
  will continue to work as before.
  [#332](https://github.com/pulumi/pulumi-kubernetes-operator/pull/332)
- Add support for referring to a [Flux source object](https://fluxcd.io/flux/components/source/api/)
  as the source of the Pulumi program to run for a Stack.
  [#324](https://github.com/pulumi/pulumi-kubernetes-operator/pull/324)
- Add functionality for using [Pulumi YAML](https://www.pulumi.com/docs/intro/languages/yaml/) in-line to create Pulumi programs.
  [#336](https://github.com/pulumi/pulumi-kubernetes-operator/pull/336)
- [internal] Port testing to Ginkgo v2 and address test flakes
  [#337](https://github.com/pulumi/pulumi-kubernetes-operator/pull/337),
  [#342](https://github.com/pulumi/pulumi-kubernetes-operator/pull/342)

## 1.9.0 (2022-10-05)

**BREAKING CHANGES**

- Deprecate watching multiple namespaces, and cross-namespace references. See the PR for how to
  adapt your configuration if you use either of these.
  [#329](https://github.com/pulumi/pulumi-kubernetes-operator/pull/329)

**Updates and bug fixes**

- Exit processing early when a stack is ready to be garbage collected
  [#322](https://github.com/pulumi/pulumi-kubernetes-operator/pull/322)
- Fix a goroutine leak [#319](https://github.com/pulumi/pulumi-kubernetes-operator/pull/319)
- Use status conditions to indicate ready/in-progress/stalled status
  [#316](https://github.com/pulumi/pulumi-kubernetes-operator/pull/316)

## 1.8.0 (2022-09-01)
- Use go 1.18 for builds
- CI update to go 1.18
- Bump to v3.39.1 of Pulumi, to support short branch names in .spec.branch after
  [pulumi/pulumi#10118](https://github.com/pulumi/pulumi/pull/10118)
- Fix `stacks_failing` prometheus metric for `Stack`s with apiVersion `v1` (https://github.com/pulumi/pulumi-kubernetes-operator/pull/308)
- Bump image and pulumi/pulumi dependencies to v3.39.0 (https://github.com/pulumi/pulumi-kubernetes-operator/pull/315/)

## 1.7.0 (2022-06-09)
- Use the first namespace from the env entry WATCH_NAMESPACE for leadership election, when the value is a list; and bail if the value is malformed [#278](https://github.com/pulumi/pulumi-kubernetes-operator/pull/278)
- Bump to Pulumi v3.34.0

## 1.6.0 (2022-04-21)
- Add `State` to `additionalPrinterColumns`
- Bump to v3.3.30 of Pulumi which should simplify recovering from pending operations (see https://github.com/pulumi/pulumi/issues/4265)

## 1.5.0 (2022-03-14)
- Use configured namespace for envRef Secrets, instead of defaulting to 'default'
- Bump pulumi/pulumi dependencies
- Use go 1.17 for builds

## 1.4.0 (2022-02-02)

**BREAKING CHANGES**
- Default namespace for resources that don't provide one is now the service accounts namespace (where the operator is likely running) instead of "default"
  To revert to the previous behavior unset `PULUMI_INFER_NAMESPACE` in the [operator pod spec](https://github.com/pulumi/pulumi-kubernetes-operator/blob/master/deploy/yaml/operator.yaml) environment variables

**Updates and Bug Fixes**
- Bump dependencies and Pulumi binary to v3.23.2 (https://github.com/pulumi/pulumi-kubernetes-operator/pull/250)

## 1.3.0 (2021-12-15)
- Add ability to resync stacks periodically [#243](https://github.com/pulumi/pulumi-kubernetes-operator/pull/243)
- Bump to Pulumi v3.19.0 [#246](https://github.com/pulumi/pulumi-kubernetes-operator/pull/246)

## 1.2.1 (2021-11-05)
- Simplified pulumi program installation instructions [#238](https://github.com/pulumi/pulumi-kubernetes-operator/pull/238)

## 1.2.0 (2021-11-04)
- Default timestamps are now iso8601 in logs. Remove `"--zap-time-encoding=iso8601"` line from deployment spec to revert to old timestamps [#234](https://github.com/pulumi/pulumi-kubernetes-operator/pull/234)
- Add some basic event publishing [#235](https://github.com/pulumi/pulumi-kubernetes-operator/pull/235)
- Upgrade to v3.17.0 of Pulumi [#236](https://github.com/pulumi/pulumi-kubernetes-operator/pull/236)

## 1.1.0 (2021-10-27)
- Avoid double install of dependencies [#230](https://github.com/pulumi/pulumi-kubernetes-operator/pull/230)
- Update to v3.16.0 of Pulumi [#230](https://github.com/pulumi/pulumi-kubernetes-operator/pull/230)

## 1.0.0 (2021-10-12)
**First GA release**
Follow installation instructions [here](https://github.com/pulumi/pulumi-kubernetes-operator#deploy-the-operator).

- Upgrade to v3.14.0 of Pulumi [#227](https://github.com/pulumi/pulumi-kubernetes-operator/pull/227)

## 1.0.0-rc1 (2021-10-11)
- Promote v1alpha1 CRD to v1 but maintain backward compatibility [#220](https://github.com/pulumi/pulumi-kubernetes-operator/pull/220)

## 0.0.22 (2021-10-11)
- Make max reconciles configurable. Users can now set `MAX_CONCURRENT_RECONCILES` to limit concurrent reconciles (defaults to 10). [#213](https://github.com/pulumi/pulumi-kubernetes-operator/pull/213/)
- Nested secret outputs are now masked by default. [#216](https://github.com/pulumi/pulumi-kubernetes-operator/pull/216/)
- Add metrics support [#217](https://github.com/pulumi/pulumi-kubernetes-operator/pull/217)

## 0.0.21 (2021-10-04)
- Fix clean up logic on reconcile [#203](https://github.com/pulumi/pulumi-kubernetes-operator/pull/203)
- Fix stack refresh for BYO backend [#200](https://github.com/pulumi/pulumi-kubernetes-operator/pull/200)
- Bump to pulumi v3.13.2 [#207](https://github.com/pulumi/pulumi-kubernetes-operator/pull/207)
- Add docs for Stack CR [#205](https://github.com/pulumi/pulumi-kubernetes-operator/pull/205)

## 0.0.20 (2021-09-27)
- Improve workdir cleanup logic [#195](https://github.com/pulumi/pulumi-kubernetes-operator/pull/195)
- Bump to Pulumi v3.13.0 [#198](https://github.com/pulumi/pulumi-kubernetes-operator/pull/198)

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
