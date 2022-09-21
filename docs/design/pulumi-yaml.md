# Integration with Pulumi YAML Programs

## Summary

This design is describes the approach to implementing the ability to execute Pulumi YAML programs from the Operator, by declaring the program as a Kubernetes resource.

## Motivation

Pulumi YAML is intended as an accessible and easy way to write Pulumi programs. 
YAML is already the standard for creating Kubernetes resources.

Therefore, combining the two means we can provide an easy way to create projects within the Operator, without requiring the use of an external repo for storing a Pulumi project. Projects can be created, edited, and deleted within Kubernetes, making use of kubectl or other automation, as with any other Kubernetes resource.

## Design

Add a new CRD to the Operator, specifying a Pulumi program. This will be [Pulumi YAML](https://www.pulumi.com/docs/reference/yaml/) (as used with the Pulumi CLI) under a 'Program' field, in addition to the usual, compulsory Kubernetes API fields. Additionally add a field to the Stack resource referencing the program, as an alternative to projectRepo (or upcoming Flux fields).

```YAML
apiVersion: pulumi.com/v1
kind: Program
metadata:
  name: example-program
program:
  #Possible requirement for Pulumi project file fields
  name:
  runtime:
  template:
  ...
  #Pulumi YAML
  configuration:
  resources:
  variables:
  outputs:
```

```YAML
apiVersion: pulumi.com/v1 #May necessitate a version bump
kind: Stack
metadata:
  name: example-stack
spec:
  program: example-program
  ...
```

## Alternatives Considered

Two main alternatives were considered: the use of exact Pulumi YAML (rather than 'wrapping' it in a Kubernetes resource), or defining the program within the stack and removing the need for a new resource altogether.

Using Pulumi YAML directly could provide a nicer user experience (and a lower barrier for entry), allowing users to use the same program spec with `pulumi up` and `kubectl apply`. However, this would have required significant work within the Pulumi YAML project itself; requiring it to understand (and appropriately handle/ignore) Kubernetes specific fields. Additionally, adding specially treated fields to the Pulumi YAML specification would likely cause confusion for users. It is also possible that in the future the Pulumi CLI could be adapted to also accept Kubernetes YAML (as defined here), allowing for program resources to be directly run from CLI.

Defining the Pulumi program within the stack would provide some simplicity to the Operator; having the stack entirely contained in a single object. This would also avoid dangling refs; a stack existing without the referenced program or vice versa. However, having the Program separated out from the stack is a cleaner architecture within Kubernetes - avoiding having a large and unwieldy single resource. This also allows for the Program to be updated without the need to touch the stack. This would also open the possibility of reusing a Program across multiple Stacks - however would also mean that updating the Program would change it in all Stacks.

## Backward Compatibility

Similarly to Flux integration, the addition of more options to how a Pulumi program may be specified to a Stack would mean making the current `projectRepo` field optional, rather than it's current required status. This would technically be a breaking change, although the repercussions of such a change would be small.

## Questions

Does the program object require Pulumi project file fields?

Does a new API version mean further changes should be made?
