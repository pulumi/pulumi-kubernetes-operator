# Using the Pulumi operator with Flux

You can refer to a [Flux source](https://fluxcd.io/flux/components/source/) from a Stack, which
tells the operator to fetch the program to run from the source, rather than directly from git.

In short: give the name, API version, and kind of the source in a `fluxSource` object, instead of
supplying `projectRepo`.

```yaml
apiVersion: pulumi.com/v1
kind: Stack
metadata:
  name: stack-from-source
spec:
  stack: squaremo/app/dev
  fluxSource:
    sourceRef:
      apiVersion: source.toolkit.fluxcd.io/v1beta2
      kind: GitRepository
      name: app-source
    dir: app/
```

## Why use the operator with Flux?

Flux sources have a few more features than the git polling built into the operator; for example,

 - you have a choice of getting programs from other kinds of source, e.g., [OCI artifacts (container
   images)](https://fluxcd.io/flux/components/source/ocirepositories/) and [S3-compatible
   buckets](https://fluxcd.io/flux/components/source/buckets/).
 - there's more flexibility in which revisions get fetched; e.g., you can follow [tags matching a
   semver range](https://fluxcd.io/flux/components/source/gitrepositories/#semver-example);
 - you can [configure webhook receivers](https://fluxcd.io/flux/guides/webhook-receivers/) so GitHub
   and other platforms can ping the cluster when there's a new commit, making updates more
   responsive;
 - it will [verify signed
   commits](https://fluxcd.io/flux/components/source/gitrepositories/#verification) and [Cosign
   signatures on OCI
   artifacts](https://fluxcd.io/flux/components/source/ocirepositories/#verification);
 - Flux will work in some environments where the git implementation matters, like Azure DevOps.

The [Flux source documentation][] gives all the options.

## What is required

You will need to install Flux on your cluster. You have a choice:

 1. bootstrap with Flux and use it to install the Pulumi operator;
 1. install Flux and the operator using Pulumi;

These alternatives are explained in the sections following.

You will also need to give the Pulumi operator RBAC permissions to access Flux source objects. A
role and role binding with the appropriate permissions is in `deploy/flux` in this git repository.

### Bootstrapping with Flux

This is what you can do if you want to maintain your cluster configuration in git and let Flux apply
it. If you are already using Flux in your cluster, this is most likely what you did.

In this scenario, using the Pulumi operator alongside Flux gives you the ability to manage the
infrastructure outside your Kubernetes cluster through git as well. You get to choose which things
within the cluster are managed by Flux, and which are managed by Pulumi -- you could apply only the
operator and its Stacks and Programs with Flux, and use Pulumi for everything else, for example.

Start by [bootstrapping Flux](https://fluxcd.io/flux/get-started/).

This will install the Flux controllers, and put a configuration for them into a git repo, then sync
from that git repo. From this point on, you can layer configuration for the cluster by committing
YAMLs to the git repo (an example is given [later in the
guide](https://fluxcd.io/flux/get-started/#add-podinfo-repository-to-flux)).

Add a GitRepository and a Kustomization to apply the YAML files for the Pulumi operator repo:

```console
flux create source git pulumi-operator-repo \
  --tag-semver '1.10.x' \
  --url 'https://github.com/pulumi/pulumi-kubernetes-operator'
flux create kustomization pulumi-operator-crds \
  --source=GitRepository/pulumi-operator-repo \
  --path="./deploy/crds"
flux create kustomization pulumi-operator \
  --source=GitRepository/pulumi-operator-repo \
  --depends-on=pulumi-operator-crds \
  --target-namespace=default --path="./deploy/yaml"
```

You can now deploy Stack and Program objects by committing them to git in the cluster configuration
repo created by Flux.

> Hint: you can use `--export` with the flux commands to put the configuration in a file to commit
> to git, rather than applying it directly.

To enable the Pulumi operator to access Flux sources, you also need the RBAC role and role binding
mentioned previously. Either commit them to your cluster configuration repo, or point another
Kustomization object at the `deploy/flux` directory. <!-- TODO this can be better! A kustomizaton
with the whole required config would be great -->

### Bootstrapping with Pulumi

You can install Flux using Pulumi. If you are using Pulumi to provision the Kubernetes cluster in
the first place, this may be the most natural way to build from there.

In this scenario, your infrastructure is created by Pulumi, and Flux serves mainly as a means for
fetching sources for the Pulumi operator to use. Here is an outline of what you would do after you
have a Kubernetes cluster:

 1. Use the [Flux provider][] in a Pulumi program to install Flux to the cluster
 2. Commit the program to git;
 3. Construct a Flux source, and a Stack object, to run the program from git.

The example `examples/flux-sources` shows how to write the program in 1.).

<!-- TODO: make the bootstrapping program include its own source and Stack (like flux bootstrap
does, effectively). -->

## Constructing a source and a Stack object
