# Using `.spec.prerequisites` in a Stack object

This example demonstrates a scenario in which you need to use the "prerequisites" mechanism.

The essence of the scenario is this: you have credentials stored in an external system, and you use
those credentials to initialise a provider. The problem that arises is that when you want to use
`.spec.refresh` in a Stack running this kind of program, it will use the credentials from its state
rather than reading them afresh; if you have rotated the credentials, those in the state will be
stale and the refresh will fail.

## Demonstrating the problem, locally

You can see how this scenario behaves by running the stack locally, rather than with the operator:

Install Vault in the cluster (or if you have access to an existing Vault instance, use its address
below rather than port-forwarding):

```bash
sh ./run-vault.sh
```

This will start a Vault server in Kubernetes (whichever cluster your current context points to). You
can use this server from the command line as well as from within the cluster. The address is
exported so you don't have to repeat it; but, we'll be supplying different access tokens, so it will
be supplied each time.

Port forward to it and store the address in `VAULT_ADDR`:

```bash
kubectl port-forward service/vault 8200:8200 &
export VAULT_ADDR=http://localhost:8200 # this assumes the port forwarding
```

If using the (dev mode) Vault in the cluster, the root token will be `root`:

```bash
vault_root_token=root
```

The Pulumi program in `./use-vault/` uses the environment variables `VAULT_ADDR` and `VAULT_TOKEN`
to create a Vault provider and enter a key/value into the vault. To show the problematic scenario,
we'll create a short-lived token to use with the Pulumi program.

```bash
vault_temp_token=$(VAULT_TOKEN=$vault_root_token vault token create -ttl=3m -format=json | jq '.auth.client_token')
```

Then,

```bash
(cd use-vault; go mod tidy) # get the program's dependencies
VAULT_TOKEN=$vault_temp_token pulumi -C ./use-vault up
```

This should work (provided you didn't wait too long before using the temp token -- just run that bit
again if `pulumi up` fails).

You should also be able to run `pulumi -C use-vault refresh` (without supplying the token, because
it was recorded in the Pulumi state). But, if you wait a few minutes, the temp token kept in the
Pulumi state will expire, and `pulumi -C use-vault refresh` will fail.

Since you're using the command line and you can create Vault access tokens, you can of course just
create a fresh token -- but note, you need to run `pulumi up` again for it to be recorded in the
state -- `pulumi refresh`, or `pulumi destroy`, or even `pulumi up --refresh`, will fail because
they all refer to the state for the provider credentials. With the Pulumi Kubernetes operator, this
is a more pronounced problem because there's no opportunity to step in and supply a fresh token.

## Demonstrating the problem using the Pulumi operator

To demonstrate how this is a problem when using the Pulumi operator, you need to create a Stack
object so the program above is run. To provide the access token, we'll put it in a Kubernetes secret
and refer to that from the stack.

This script creates a fresh token as above, and creates or recreates the Kubernetes secret:

```bash
sh ./rotate-key.sh $vault_root_token
```

Now we can create the stack itself:

```bash
kubectl apply -f stack.yaml
```

You can watch this succeed (hopefully!) with kubectl:

```bash
kubectl get stack -w
```

(If it doesn't succeed, it might be because the token expired before the stack could run far enough. Delete
the stack and the secret, create a fresh token and go from there.)

Since the stack has `.spec.refresh` set (the equivalent of `pulumi up --refresh`), eventually the
token will expire and it will start failing. Updating the secret will not help, because the stack
will fail at refreshing, and never get as far as running the program and using the new token.

## Solving the problem with a targeted stack

We can create a stack to refresh the token -- this one will update only the state of the provider,
i.e., the credentials used. Once that is updated, the original stack will be able to refresh.

The file `prereq.yaml` is derived from `stack.yaml`, but does _not_ set `.spec.refresh` (otherwise
it would have the same problem as `stack.yaml`); and, it includes the field `.spec.targets`, which
tells the operator to update only the state for the the resources specified. In this case, the URN
given identifies the provider resource.

> The URN is already filled in for this example, but in general you would get a resource's URN by
> looking at the stack state. If using the [Pulumi service](https://app.pulumi.com), click on a
> resource in the stack view to see its URN. If using another backend, you can use `pulumi stack
> --show-urns --stack local`. When using a `file://` backed with a Stack object, you need to execute
> a shell in the operator's pod to do this.

Create a fresh token, and apply the stack:

```bash
sh ./rotate-key.sh $vault_root_token
kubectl apply -f prereq.yaml
```

This stack should succeed; and subsequently, so should the origin stack (called `use-vault`).

Lastly, to make sure that the targeted stack will have updated the credentials whenever the original
stack runs, we can put it as a "prerequisite" of the original stack. The file
`stack-with-prereq.yaml` gives the same Stack object as `stack.yaml`, but adds a prerequisite into
`.spec`:

```yaml
spec:
    # ...
    prerequisites:
    - name: update-vault-creds
      requirement:
          succeededWithinDuration: 5m
```

The prerequisite says "before this runs, make sure the stack 'update-vault-creds' has succeeded in
the last five minutes, running it first if necessary".

## Summary

The problem to be solved is that when you create a provider explicitly in your program, its
parameters are captured in the stack state; and, since these might be credentials that expire, this
will mean eventually, you won't be able to refresh or delete the stack.

The general solution to this is to run the stack with the provider resource as the target, so its
parameters are re-evaluated. When using the operator, you can use `.spec.prerequisites` to make sure
another stack has run before this one; and, in the prerequisite, `.spec.target` to just update the
provider resource.
