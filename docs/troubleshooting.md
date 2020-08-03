# Troubleshooting

* If a Stack is stuck due to conflicting updates from a previous failed run,
you'll need to have the Pulumi program accessible locally, and manually cancel the stack.

  ```bash
  pulumi stack select dev
  pulumi cancel -y
  ```
  
  Once cancelled, retry creating the Stack CustomResource.

* If your Stack CR encounters an error and is not processed, the operator by
will still continue to deploy reconciliation loops until a successful update is reached.

  In these cases it's best to delete the Stack CR and redeploy it.
  
  If `destroyOnFinalize: true` is set, you first have to remove its
  [finalizer](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/) before the Stack CR can be deleted.
  
  e.g.
  
  ```bash
  kubectl patch stack my-stack-0q4s6z9z -p '{"metadata":{"finalizers": []}}' --type=merge
  ```
