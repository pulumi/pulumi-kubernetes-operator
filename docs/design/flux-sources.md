# Integration with Flux sources

## Summary

This design shows how to integrate the Pulumi Kubernetes operator with [Flux
sources](https://fluxcd.io/flux/components/source/).

## Motivation

[Flux](https://fluxcd.io/) is a set of Kubernetes controllers that implement GitOps automation. The
core controllers fetch configuration from a git repository or other such source, and apply it into
the cluster. Auxiliary controllers handle incoming webhooks (e.g., from GitHub), and send outbound
notifications (e.g., to Slack).

Broadly speaking, integration with Flux would bring in features that would be expensive to replicate
in the Pulumi operator. And since the Flux controllers can be used individually, it's possible to
integrate with selected parts. This design targets the source controller, which handles fetching
configurations from git repositories, S3 buckets, and OCI (container image) registries.

The benefit of using this integration will be:

 - it works around some limitations of the go-git-based git fetching built into the operator (via
   automation API) -- specifically, it cannot work with Azure DevOps and some other platforms
 - it becomes possible to use Flux's notification-controller and webhooks to make the operator more
   responsive to new commits
 - users get access to useful features, e.g., you can follow tags according to a semver range, as
   well as branches; exclude files from the source; verify source revisions using signatures; and so
   on
 - users get access to sources other than git repositories: S3 buckets and OCI images
 - sources and stacks can be specified by different teams, since they are different objects and can
   have different RBAC rules in effect.

The main disadvantage to using the integration is that you have to run the Flux source controller,
which is an additional installation step, and another moving part. It is at least simple to install
and self-contained; that is, there are no dependencies you also need to install and run.

## Design

How this should work mechanically is pretty obvious: you refer to a source in your Stack object,
rather than suppling git particulars, and the controller fetches from the referenced source object
rather than from git directly.

### Stack spec

The slightly tricky bit is how to fit this into the Stack API. The git particulars are fields
directly under `.spec`, so it is not possible cleanly to use the "union" convention, in which the
fields would be nested in another struct, looking a bit like a tagged union:

```YAML
spec:
  git:
    projectRepo: ...
    gitAuth:
      ...
    repoDir: ./project
```

They can't be moved without manufacturing another version of the API -- probably `v2alpha1` -- which
is a significant undertaking.

However, there's not that many fields in question -- just `.spec.projectRepo`,
`.spec.{commit,branch}` which are mutually exclusive, and `.spec.gitAuth` and its deprecated
predecessor `.spec.gitAuthSecret`. The fields `.spec.repoDir` and
`.spec.continueResyncOnCommitMatch` can apply to any source, and can remain outside the "union". If
any of that initial set are present _as well as_ sourceRef, the spec is invalid (this can be checked
in the same way `.branch` and `.commit` are made mutually exclusive, in the controller code).

This is how a source reference would look:

```yaml
spec:
  sourceRef:
    name: repo
  repoDir: ./project
```

### Stack status

The status type `StackUpdateState` includes the fields `.lastAttemptedCommit` and
`.lastSuccessfulCommit`. Since Flux sources can refer to S3 buckets and OCI artifacts, they come
with a `revision` which may or may not signify a git commit. The choices are:

 - put the source revision value in the `.lastAttemptedCommit` and `.lastSuccessfulCommit` fields

This means they will have a value, but it may not be in the format expected. Automation that doesn't
understand the source revisions may break.
 
 - add fields which hold the last attempted and last successful source revision, and populate it
   _and_ the last commit (when there is a commit, specifically).

This means the commit fields will be in the right format, but may not always have a
value. Automation that understands the source revisions can consult those fields; automation that
doesn't may misinterpret the lack of a commit value.

## Questions

**Should you be able to refer to sources in other namespaces?**

It's safest not to support this, at least to start with, because allowing it sidesteps the
assumption of namespace isolation. Flux breaks the namespace isolation convention by letting you
refer to sources in other namespaces, but has a (provisional) mechanism for limiting access.
