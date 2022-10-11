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
rather than supplying git particulars, and the controller fetches from the referenced source object
rather than from git directly.

### Stack spec

The slightly tricky bit is how to fit this into the Stack API. The git particulars to which the
sourceRef is an alternative are fields directly under `.spec`, so it is not possible cleanly to use
[the "union"
convention](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#unions),
in which the fields would be nested in another struct, looking a bit like a tagged union.

```YAML
# how it _should_ look, **but won't**
spec:
  git:                 # )
    projectRepo: ...   # )
    gitAuth:           # ) this stanza;
      # ...            # ) OR,

  source:              # ) this stanza
    sourceRef:         # )
      name: foo        # )

  repoDir: ./project # then this field
```

They can't be moved without manufacturing another version of the API -- probably `v2alpha1` -- which
is a significant undertaking (there's a Question about that, below).

However, there's not that many fields in question -- just `.spec.projectRepo`, then
`.spec.{commit,branch}` which are optional (though you must supply one), and `.spec.gitAuth` and its
deprecated predecessor `.spec.gitAuthSecret`, both optional. It's possible to still treat them as a
an alternative to `sourceRef` without nesting them.

Generally, adding an alternative to a union is backward-compatible ([see the entry in API change
"gotchas"](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api_changes.md#backward-compatibility-gotchas)). In
this case though, the union convention is _not_ in place already -- specifically, the field
`.spec.projectRepo` is required (and secondarily, as mentioned the fields pertaining to git are all
at the top level, rather than nested, but that's more an inconvenience than a blocker). On the other
hand, it's hard to contrive a situation in which making `.spec.projectRepo` optional breaks existing
code. Existing definitions will still pass validation. If old controller code receives a new
definition with `.projectRepo` omitted, it will parse it as having an empty value, and fail in the
same way as it would with a non-empty but invalid value. So: making it optional and using
`omitempty` is OK in practice.

The fields `.spec.repoDir` applies to both git as present now, and to sources, but it may not apply
to alternatives (e.g., references to inline programs); so, it becomes part of the individual source
spec (`.repoDir` for a git source, `dir` for a Flux source).

The field `.spec.continueResyncOnCommitMatch` can apply to any source, so should remain outside the
"union" as well. In future API versions it should be renamed to something less specific.

If any of that original set of fields are present _as well as_ sourceRef, the spec is invalid (this
can be checked in the same way `.branch` and `.commit` are made mutually exclusive, in the
controller code).

```yaml
# how it will look
spec:
  projectRepo: ...   # )
  gitAuth:           # ) these,
    # ...            # )
  repoDir: ./project # ) OR

  fluxSource:        # ) this.
    sourceRef:       # )
      name: foo      # )
    dir: ./project   # )
  continueResyncOnCommitMatch: true
  # ... and the rest
```

### Stack status

The status type `StackUpdateState` includes the fields `.lastAttemptedCommit` and
`.lastSuccessfulCommit`. Since Flux sources can refer to S3 buckets and OCI artifacts, they come
with a `revision` which may or may not signify a git commit. The choices are:

 a. put the source revision value in the `.lastAttemptedCommit` and `.lastSuccessfulCommit` fields

This means they will have a value, but it may not be in the format expected. Automation that doesn't
understand the source revisions may break.
 
 b. add fields which hold the last attempted and last successful source revision, and populate it
   _and_ the last commit (when there is a commit, specifically).

This means the commit fields will be in the right format, but may not always have a
value. Automation that understands the source revisions can consult those fields; automation that
doesn't may misinterpret the lack of a commit value.

For the sake of fewer API type changes, I will use a.). Automation that assumes the commit string is
a git SHA1 may break, and we should warn about that. Automation that treats it as an opaque value
will be fine.

### Implementation notes

It's possible to get a referenced object as an `unstructured.Unstructured` value and examine its
status to see whether the source it represents is available; then, to fetch that source as a tarball
and expand it. None of that needs to depend on Flux modules. However, watching referenced sources
may be trickier -- I'm not clear whether it's possible (or convenient) to watch individual objects
with controller-runtime. If not, it may be necessary to watch and index the "known" source types,
and to restrict the kinds that can be used in `.spec.sourceRef`.

## Backward compatibility

See ["Stack spec"](#stack_spec) above, regarding API changes.

## Alternatives considered

**Replicating the desired features in the operator**

Rather than integrating with Flux sources, we could decide which of the features therein are
desirable and implement those within the Pulumi operator.

The benefits are

 - the operator remains self-contained
 - if (some of) the work was done in the automation API, it would be available to other Pulumi
   tooling, and users of the automation API.

Downsides:

 - the duplication of effort makes it an expensive trade-off
 - it lacks the "tie in" property of an integration
 - it makes the API less accessible to some of the intended audience (those who are familiar with
   Flux) and in any case, non-transferable knowledge

## Questions

**Should you be able to refer to sources in other namespaces?**

It's safest not to support this, at least to start with, because allowing it sidesteps the
assumption of namespace isolation. Flux breaks the namespace isolation convention by letting you
refer to sources in other namespaces, but has a (provisional) mechanism for limiting access.

**Does this need another API version?**

It would be cleaner to add `sourceRef` to another API version, for a couple of reasons:

 - the "union" convention can be used, i.e., `.spec.projectRepo` field is put in a nested, optional
   struct;
 - adding experimental features to a stable API is generally frowned upon.
