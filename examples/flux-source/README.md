# Example of using the Pulumi Kubernetes operator with Flux

The Pulumi operator can use [Flux sources][] for getting Pulumi programs. This example shows how to
set the operator, and Flux source controller, up; and, how to create a `GitRepository` (a Flux
source) and a `Stack` object that points to it.

## Running the installation program

The project in this directory will:

 - install the Flux source CRDs and controller
 - install the Pulumi operator and its CRD(s)
 - give the Pulumi operator permissions to see Flux source objects

You'll need to install the program dependencies:

```console
flux-source$ npm install
```

Now you can run the stack with the Pulumi command-line:

```console
flux-source$ pulumi up
```

## Running the example app Stack

You will need to install the dependencies for the program:

```console
app$ npm install
```

For the project in `app/`, you'll need a Pulumi access token. You can create one in the [Pulumi
service][] under "Access tokens". Assign it to the environment variable
`PULUMI_ACCESS_TOKEN`.

To run the program:

```console
app$ pulumi up
```

This creates a `GitRepository` object in the cluster, which you can examine:

```console
kubectl get gitrepository
```

and a `Stack` object, pointing at the `GitRepository` object, which will run the program and create
a deployment. You watch the stack object until it has completed:

```console
app$ kubectl get stack -w
```

You should see the state become `succeeded` in a minute or so. The result of the stack is a
deployment:

```console
app$ kubectl get deployment
```

You can also look in the [Pulumi service][] to see what it creates.

## Installing development versions

You can use the install program to run a locally-built image, with the following steps.

 * install the CRD over the top of that installed by the stack

```console
pulumi-kubernetes-operator$ make install-crds
```

 * build a development version of the image:

```console
pulumi-kubernetes-operator$ make build-image IMAGE_NAME=pulumi/pulumi-kubernetes-operator
```

(`IMAGE_NAME` means it won't name the image after `$(whoami)`)

> If you're using `kind` to run a local Kubernetes cluster, you will need to side-load the image:
>
> ```console
> kind load docker-image pulumi/pulumi-kubernetes-operator:$(git rev-parse --short HEAD)
> ```

 * use that image's tag as the `operator-version` configuration item when running the stack:

```console
pulumi-kubernetes-operator/examples/flux-source$ pulumi up -c operator-version=$(git rev-parse --short HEAD)
```

[Flux source]: https://fluxcd.io/flux/components/source/api/
[Pulumi service]: https://app.pulumi.com/
