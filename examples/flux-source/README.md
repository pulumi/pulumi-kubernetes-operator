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

You will also need to supply a git repository URL. You can use my example:
`https://github.com/squaremo/pko-dev`, but you won't be able to install webhooks there, so it might
pay to fork that or create your own. (If you do create your own, you will likely want to change the
`dir` in the Stack spec, which gives the subdirectory of the repo where your Pulumi code is).

To run the program:

```console
app$ pulumi config set git-url https://github.com/squaremo/pko-dev
app$ pulumi config set stack-name $(pulumi whoami)/app/dev
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

## Webhooks

The program in `app/` also creates a webhook receiver, so you can get notifications from GitHub when
there are new commits pushed. You will need to expose the webhook service to the internet for this
to work: either use the guide in
https://fluxcd.io/flux/guides/webhook-receivers/#expose-the-webhook-receiver, or use something like
ngrok to tunnel the webhook into a private cluster. If you use the Flux guide, be aware that the
webhook receiver here is in `default` namespace, rather than `flux-system`. There's a working
example for ngrok in `ngrok/`.

### Using ngrok to tunnel webhooks

The directory `ngrok/` has a program that will run ngrok in such a way as to make the webhooks
work. It doesn't need any configuration; just:

```console
ngrok$ pulumi up
```

To see the public hostname that it's set up, access the ngrok console:

```console
ngrok$ NGROK_SERVICE=$(pulumi stack output service)
ngrok$ kubectl port-forward service/$NGROK_SERVICE 4040:80
```

The ngrok console will then be reachable on http://localhost:4040/.

Bear in mind that the tunnel hostname will change every time this restarts, so it's only really
useful for (shortlived) demos.

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
