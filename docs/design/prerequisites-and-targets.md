# Prerequisites and targets

## Summary

This presents a design for two mechanisms that can be combined to solve the "stale provider state"
problem described in [#299](https://github.com/pulumi/pulumi-kubernetes-operator/issues/299).

## Motivation

Some Pulumi programs are structured in phases, such that inputs obtained in phase `N` are used to
construct a _provider_ for creating resources in phase `N+1`. This presents a problem for the Pulumi
machinery, because inputs to the provider must be kept in the stack state, and will become stale if
the resource in question changes; e.g., if they are credentials and are rotated. Issuing a `pulumi
refresh` (or `pulumi up -r`, or Stack object with `.spec.refresh` set) will balk in that situation,
because refreshing consults the state rather than running the program, so it will proceed with stale
credentials and likely fail authentication.

Usually it is possible to work around this by using `pulumi up -t <provider URN>` to rerun the
program and update the inputs to the provider, before running the refresh operation. However, this
workaround isn't available when using the operator, because there is no representation of "run this
before" or targeted updates in its API.

## Design

This design involves two additions to the API:

 - "prerequisites", a list of other stack objects that must be run before the stack under
   consideration is run; and,
 - "targets", a list of URNs to be given to the Pulumi engine when running the stack.

### Prerequisites

The field `.spec.prerequisites` gives a list of references to stacks. The controller makes sure each
has been run recently before running the stack under consideration. This means, for instance, you
can update credentials in the state before re-running a program that uses those credentials.

Usually it will be important that a prerequisite has run recently. The field
`.spec.prerequisite[].updatedWithinDuration` puts a limit on how long ago it was last run:

```
spec:
  prerequisites:
  - stackRef:
      name: update-credentials
    unless:
      updatedWithinDuration: 1m
```

If the referenced stack has not been run in the last `updatedWithinDuration` duration, it will be
queued, and the current stack requeued.

**Q. How does the controller queue the prerequisites?**

One mechanism is to simply add an annotation to the object, which will trigger an update event and
notify the controller (though the controller would have to be adapted to not filter those
changes).

This mechanism is used neatly by Flux to queue dependent objects; the CLI also employs it to make
interactions synchronous (mark an object with the annotation, then wait for its status to report
that it was processed).

**Q. Can you refer to the same stack in two Stack objects?**

For this to be useful for updating stale state, the depended-upon Stack must update the state used
by the depending Stack (or the program must be rewritten to use stack references, which arguably
would be good practice anyway). This would require it to have the same backend and name the same
stack; this is likely to be possible, but may cause conflicts if not carefully managed.

Note that it is not _always_ a requirement that a prerequisite uses the same stack or backend --
that is how you use it to e.g., update rotated credentials, if that's done all in one program.

### Targets

The field `.spec.targets` has a list of URNs to supply to the Pulumi automation API when running the
stack.

## How this design solves the stale provider state problem

To solve the stale provider state problem, you would make two stacks:

 - the first targets the provider URN
 - the second runs the program as usual, and refers to the first as a prerequisite.

The effect is that the provider state is updated before the "main" program runs, so it will have
up-to-date credentials.

## Alternatives considered

### Supplying prerequisite targets in one field

This formulation works similarly to the design given above, but combines the dependence with the
target:

```
# ...
spec:
    updateBeforeRefresh:
    - <URN>
    - <URN>
```

The implementation in the controller runs the equivalent of `pulumi up -t <URN>...` before
continuing on to running the stack without targets.

Combining the two mechanisms means they cannot be used on their own, or with separate stacks (and
stack references), which is a less worthwhile trade-off for the additional complexity.

## Backward compatibility

Features gates will be used so that the two mechanisms must explicitly be enabled. This surfaces the
situation in which you have an old controller and expect new behaviour, since an old controller will
not understand the feature gate flags, and refuse to run.

|    | Old API | New API |
|----|---------|---------|
| Old controller | As before. | Will behave like new controller without feature gates |
| New controller | Same behaviour as old controller | New features work when feature gates are open |
