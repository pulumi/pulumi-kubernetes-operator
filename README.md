# Pulumi Kubernetes Operator

A simple operator that deploys Pulumi updates by cloning Git repos and running Pulumi programs. This is
a hacky prototype to explore the idea, experimental, and not ready for prime time.

## Overview

- [Operator Design Doc](https://docs.google.com/document/d/1cXsamgIbiF7QDXz4mQ7tBpowUt6vgdXY1ZpRcCwF9Pk/edit#)
- Kubernetes Custom Resource Definitions (CRD): A custom Kubernetes API type.
- Kubernetes Custom Resources (CR) - an instance of a Custom Resource Definition.

## Project Layout

### Stack CRD and CR

A custom Kubernetes API type to implement a Pulumi Stack, it's configuration,
and job run settings.

- [CRD API type](./pkg/apis/pulumi/v1alpha1/stack_types.go)
- [Generated CRD YAML Manifest](./deploy/crds/pulumi.com_stacks_crd.yaml)
- [Generated CR YAML Manifest](./deploy/crds/pulumi.com_v1alpha1_stack_cr.yaml)

### Stack Controller

A controller that registers the Stack CRD, and manages user-created CRs by
creating a Kubernetes Job for each Stack CR that is attempted to run until
completion of the Pulumi update execution run.

- [Stack Controller](./pkg/controller/stack/stack_controller.go)

### Operator

A managed Kubernetes application that uses the Stack Controller to operate the
Stack CRD and user-created Stack CRs.

- [Deployment - Generated YAML Manifest](./deploy/operator.yaml)
- [Role - Generated YAML Manifest](./deploy/role.yaml)
- [RoleBinding - Generated YAML Manifest](./deploy/role_binding.yaml)
- [ServiceAccount - Generated YAML Manifest](./deploy/service_account.yaml)

## Official Operator SDK Docs

- [Quickstart](https://sdk.operatorframework.io/docs/golang/quickstart/)
- [Project Scaffolding](https://sdk.operatorframework.io/docs/golang/references/project-layout/)

## Walkthrough

Install the [`operator-sdk`][operator-sdk] to build and run locally.  

Ensure generated CRDs and controller logic is up to date by running:

```
$ make build
```

Install the CRD in your cluster.  

```
$ make install-crds
```

To build and install together, simply run `make`.

The CRD currently assumes that a stack already exists (and drives deployments against that existing stack).  So ensure that you have an existing GitHub repo, a stack created for the same project as the contents of the repo, and update `examples/s3_bucket_stack.yaml` to refer to the appropriate github `project` and `commit`, Pulumi `stack` name, and appropriate Pulumi `accessToken` and environment variables needed for the deployment controller.  Once these are ready - deploy the `pulumi.com/v1alpha1.Stack`:

```
$ kubectl apply -f examples/s3_bucket_stack.yaml
```

Push the built image to DockerHub.

```bash
$ make push-image
```

Deploy the controller locally.

```bash
$ make deploy
```

<details>
<summary>Click to expand output</summary>

```bash
INFO[0000] Running the operator locally; watching namespace "default" 
{"level":"info","ts":1592887969.2161229,"logger":"cmd","msg":"Operator Version: 0.0.1"}
{"level":"info","ts":1592887969.216164,"logger":"cmd","msg":"Go Version: go1.14.4"}
{"level":"info","ts":1592887969.2161689,"logger":"cmd","msg":"Go OS/Arch: darwin/amd64"}
{"level":"info","ts":1592887969.2161732,"logger":"cmd","msg":"Version of operator-sdk: v0.17.0"}
{"level":"info","ts":1592887969.217471,"logger":"leader","msg":"Trying to become the leader."}
{"level":"info","ts":1592887969.21748,"logger":"leader","msg":"Skipping leader election; not running in a cluster."}
{"level":"info","ts":1592887969.677693,"logger":"controller-runtime.metrics","msg":"metrics server is starting to listen","addr":"0.0.0.0:8383"}
{"level":"info","ts":1592887969.677797,"logger":"cmd","msg":"Registering Components."}
{"level":"info","ts":1592887969.6778612,"logger":"cmd","msg":"Skipping CR metrics server creation; not running in a cluster."}
{"level":"info","ts":1592887969.677867,"logger":"cmd","msg":"Starting the Cmd."}
{"level":"info","ts":1592887969.677962,"logger":"controller-runtime.manager","msg":"starting metrics server","path":"/metrics"}
{"level":"info","ts":1592887969.6780028,"logger":"controller-runtime.controller","msg":"Starting EventSource","controller":"stack-controller","source":"kind source: /, Kind="}
{"level":"info","ts":1592887969.781933,"logger":"controller-runtime.controller","msg":"Starting Controller","controller":"stack-controller"}
{"level":"info","ts":1592887969.7819672,"logger":"controller-runtime.controller","msg":"Starting workers","controller":"stack-controller","worker count":1}
{"level":"info","ts":1592887969.782099,"logger":"controller_stack","msg":"Reconciling Stack","Request.Namespace":"default","Request.Name":"s3-bucket-stack"}
{"level":"info","ts":1592887969.886507,"logger":"controller_stack","msg":"Cloning Stack repo","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Stack.Name":"lukehoban/dev","Stack.Repo":"https://github.com/joeduffy/test-s3-op-project","Stack.Commit":"3edeafe930e2121358f56c7a9adc41f18505149e","Stack.Branch":""}
{"level":"info","ts":1592887970.7098439,"logger":"controller_stack","msg":"Running Pulumi command","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Args":["stack","select","lukehoban/dev"],"Workdir":"/var/folders/yg/4frmdt6j7qj02r6nm7ftdt2w0000gn/T/960045512"}
{"level":"info","ts":1592887973.565784,"logger":"controller_stack","msg":"NPM/Yarn","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/usr/local/bin/npm","Args":["/usr/local/bin/npm","install"],"Stdout":""}
{"level":"info","ts":1592887973.56596,"logger":"controller_stack","msg":"NPM/Yarn","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/usr/local/bin/npm","Args":["/usr/local/bin/npm","install"],"Stdout":"> @pulumi/docker@2.1.1 install /private/var/folders/yg/4frmdt6j7qj02r6nm7ftdt2w0000gn/T/960045512/node_modules/@pulumi/docker"}
{"level":"info","ts":1592887973.5659678,"logger":"controller_stack","msg":"NPM/Yarn","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/usr/local/bin/npm","Args":["/usr/local/bin/npm","install"],"Stdout":"> node scripts/install-pulumi-plugin.js resource docker v2.1.1"}
{"level":"info","ts":1592887973.5659719,"logger":"controller_stack","msg":"NPM/Yarn","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/usr/local/bin/npm","Args":["/usr/local/bin/npm","install"],"Stdout":""}
{"level":"info","ts":1592887973.67189,"logger":"controller_stack","msg":"NPM/Yarn","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/usr/local/bin/npm","Args":["/usr/local/bin/npm","install"],"Text":"[resource plugin docker-2.1.1] installing"}
{"level":"info","ts":1592887973.6778262,"logger":"controller_stack","msg":"NPM/Yarn","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/usr/local/bin/npm","Args":["/usr/local/bin/npm","install"],"Stdout":""}
{"level":"info","ts":1592887973.677848,"logger":"controller_stack","msg":"NPM/Yarn","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/usr/local/bin/npm","Args":["/usr/local/bin/npm","install"],"Stdout":"> @pulumi/aws@2.4.0 install /private/var/folders/yg/4frmdt6j7qj02r6nm7ftdt2w0000gn/T/960045512/node_modules/@pulumi/aws"}
{"level":"info","ts":1592887973.677879,"logger":"controller_stack","msg":"NPM/Yarn","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/usr/local/bin/npm","Args":["/usr/local/bin/npm","install"],"Stdout":"> node scripts/install-pulumi-plugin.js resource aws v2.4.0"}
{"level":"info","ts":1592887973.677883,"logger":"controller_stack","msg":"NPM/Yarn","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/usr/local/bin/npm","Args":["/usr/local/bin/npm","install"],"Stdout":""}
{"level":"info","ts":1592887973.78512,"logger":"controller_stack","msg":"NPM/Yarn","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/usr/local/bin/npm","Args":["/usr/local/bin/npm","install"],"Text":"[resource plugin aws-2.4.0] installing"}
{"level":"info","ts":1592887973.798809,"logger":"controller_stack","msg":"NPM/Yarn","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/usr/local/bin/npm","Args":["/usr/local/bin/npm","install"],"Stdout":""}
{"level":"info","ts":1592887973.7988522,"logger":"controller_stack","msg":"NPM/Yarn","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/usr/local/bin/npm","Args":["/usr/local/bin/npm","install"],"Stdout":"> protobufjs@6.9.0 postinstall /private/var/folders/yg/4frmdt6j7qj02r6nm7ftdt2w0000gn/T/960045512/node_modules/protobufjs"}
{"level":"info","ts":1592887973.798866,"logger":"controller_stack","msg":"NPM/Yarn","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/usr/local/bin/npm","Args":["/usr/local/bin/npm","install"],"Stdout":"> node scripts/postinstall"}
{"level":"info","ts":1592887973.798869,"logger":"controller_stack","msg":"NPM/Yarn","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/usr/local/bin/npm","Args":["/usr/local/bin/npm","install"],"Stdout":""}
{"level":"info","ts":1592887973.9024699,"logger":"controller_stack","msg":"NPM/Yarn","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/usr/local/bin/npm","Args":["/usr/local/bin/npm","install"],"Text":"npm WARN s3-op-project@ No description"}
{"level":"info","ts":1592887973.902626,"logger":"controller_stack","msg":"NPM/Yarn","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/usr/local/bin/npm","Args":["/usr/local/bin/npm","install"],"Text":"npm WARN s3-op-project@ No repository field."}
{"level":"info","ts":1592887973.9027798,"logger":"controller_stack","msg":"NPM/Yarn","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/usr/local/bin/npm","Args":["/usr/local/bin/npm","install"],"Text":"npm WARN s3-op-project@ No license field."}
{"level":"info","ts":1592887973.9029682,"logger":"controller_stack","msg":"NPM/Yarn","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/usr/local/bin/npm","Args":["/usr/local/bin/npm","install"],"Text":""}
{"level":"info","ts":1592887973.903899,"logger":"controller_stack","msg":"NPM/Yarn","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/usr/local/bin/npm","Args":["/usr/local/bin/npm","install"],"Stdout":"added 105 packages from 223 contributors and audited 105 packages in 2.33s"}
{"level":"info","ts":1592887973.943458,"logger":"controller_stack","msg":"NPM/Yarn","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/usr/local/bin/npm","Args":["/usr/local/bin/npm","install"],"Stdout":""}
{"level":"info","ts":1592887973.9435341,"logger":"controller_stack","msg":"NPM/Yarn","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/usr/local/bin/npm","Args":["/usr/local/bin/npm","install"],"Stdout":"15 packages are looking for funding"}
{"level":"info","ts":1592887973.943567,"logger":"controller_stack","msg":"NPM/Yarn","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/usr/local/bin/npm","Args":["/usr/local/bin/npm","install"],"Stdout":"  run `npm fund` for details"}
{"level":"info","ts":1592887973.94357,"logger":"controller_stack","msg":"NPM/Yarn","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/usr/local/bin/npm","Args":["/usr/local/bin/npm","install"],"Stdout":""}
{"level":"info","ts":1592887973.944107,"logger":"controller_stack","msg":"NPM/Yarn","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/usr/local/bin/npm","Args":["/usr/local/bin/npm","install"],"Stdout":"found 0 vulnerabilities"}
{"level":"info","ts":1592887973.944135,"logger":"controller_stack","msg":"NPM/Yarn","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/usr/local/bin/npm","Args":["/usr/local/bin/npm","install"],"Stdout":""}
{"level":"info","ts":1592887973.9695702,"logger":"controller_stack","msg":"Running Pulumi command","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Args":["up","--skip-preview","--yes"],"Workdir":"/var/folders/yg/4frmdt6j7qj02r6nm7ftdt2w0000gn/T/960045512"}
{"level":"info","ts":1592887974.2847989,"logger":"controller_stack","msg":"Pulumi CLI","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/Users/lukehoban/.pulumi/bin/pulumi","Args":["pulumi","--non-interactive","up","--skip-preview","--yes"],"Stdout":"Updating (dev):"}
{"level":"info","ts":1592887974.694409,"logger":"controller_stack","msg":"Pulumi CLI","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/Users/lukehoban/.pulumi/bin/pulumi","Args":["pulumi","--non-interactive","up","--skip-preview","--yes"],"Stdout":""}
{"level":"info","ts":1592887975.130214,"logger":"controller_stack","msg":"Pulumi CLI","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/Users/lukehoban/.pulumi/bin/pulumi","Args":["pulumi","--non-interactive","up","--skip-preview","--yes"],"Stdout":"    pulumi:pulumi:Stack s3-op-project-dev running "}
{"level":"info","ts":1592887980.355218,"logger":"controller_stack","msg":"Pulumi CLI","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/Users/lukehoban/.pulumi/bin/pulumi","Args":["pulumi","--non-interactive","up","--skip-preview","--yes"],"Stdout":"    aws:s3:Bucket my-bucket  "}
{"level":"info","ts":1592887980.6905558,"logger":"controller_stack","msg":"Pulumi CLI","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/Users/lukehoban/.pulumi/bin/pulumi","Args":["pulumi","--non-interactive","up","--skip-preview","--yes"],"Stdout":"    pulumi:pulumi:Stack s3-op-project-dev  "}
{"level":"info","ts":1592887980.690592,"logger":"controller_stack","msg":"Pulumi CLI","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/Users/lukehoban/.pulumi/bin/pulumi","Args":["pulumi","--non-interactive","up","--skip-preview","--yes"],"Stdout":" "}
{"level":"info","ts":1592887980.690609,"logger":"controller_stack","msg":"Pulumi CLI","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/Users/lukehoban/.pulumi/bin/pulumi","Args":["pulumi","--non-interactive","up","--skip-preview","--yes"],"Stdout":"Outputs:"}
{"level":"info","ts":1592887980.690614,"logger":"controller_stack","msg":"Pulumi CLI","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/Users/lukehoban/.pulumi/bin/pulumi","Args":["pulumi","--non-interactive","up","--skip-preview","--yes"],"Stdout":"    bucketName: \"my-bucket-b6197b5\""}
{"level":"info","ts":1592887980.690687,"logger":"controller_stack","msg":"Pulumi CLI","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/Users/lukehoban/.pulumi/bin/pulumi","Args":["pulumi","--non-interactive","up","--skip-preview","--yes"],"Stdout":""}
{"level":"info","ts":1592887980.690698,"logger":"controller_stack","msg":"Pulumi CLI","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/Users/lukehoban/.pulumi/bin/pulumi","Args":["pulumi","--non-interactive","up","--skip-preview","--yes"],"Stdout":"Resources:"}
{"level":"info","ts":1592887980.6907032,"logger":"controller_stack","msg":"Pulumi CLI","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/Users/lukehoban/.pulumi/bin/pulumi","Args":["pulumi","--non-interactive","up","--skip-preview","--yes"],"Stdout":"    2 unchanged"}
{"level":"info","ts":1592887980.690707,"logger":"controller_stack","msg":"Pulumi CLI","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/Users/lukehoban/.pulumi/bin/pulumi","Args":["pulumi","--non-interactive","up","--skip-preview","--yes"],"Stdout":""}
{"level":"info","ts":1592887980.6908128,"logger":"controller_stack","msg":"Pulumi CLI","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/Users/lukehoban/.pulumi/bin/pulumi","Args":["pulumi","--non-interactive","up","--skip-preview","--yes"],"Stdout":"Duration: 6s"}
{"level":"info","ts":1592887980.690819,"logger":"controller_stack","msg":"Pulumi CLI","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/Users/lukehoban/.pulumi/bin/pulumi","Args":["pulumi","--non-interactive","up","--skip-preview","--yes"],"Stdout":""}
{"level":"info","ts":1592887980.8378198,"logger":"controller_stack","msg":"Pulumi CLI","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/Users/lukehoban/.pulumi/bin/pulumi","Args":["pulumi","--non-interactive","up","--skip-preview","--yes"],"Stdout":"Permalink: https://app.pulumi.com/lukehoban/s3-op-project/dev/updates/6"}
{"level":"info","ts":1592887980.840992,"logger":"controller_stack","msg":"Running Pulumi command","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Args":["stack","output","--json"],"Workdir":"/var/folders/yg/4frmdt6j7qj02r6nm7ftdt2w0000gn/T/960045512"}
{"level":"info","ts":1592887981.194305,"logger":"controller_stack","msg":"Pulumi CLI","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/Users/lukehoban/.pulumi/bin/pulumi","Args":["pulumi","--non-interactive","stack","output","--json"],"Stdout":"{"}
{"level":"info","ts":1592887981.19438,"logger":"controller_stack","msg":"Pulumi CLI","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/Users/lukehoban/.pulumi/bin/pulumi","Args":["pulumi","--non-interactive","stack","output","--json"],"Stdout":"  \"bucketName\": \"my-bucket-b6197b5\""}
{"level":"info","ts":1592887981.1943848,"logger":"controller_stack","msg":"Pulumi CLI","Request.Namespace":"default","Request.Name":"s3-bucket-stack","Path":"/Users/lukehoban/.pulumi/bin/pulumi","Args":["pulumi","--non-interactive","stack","output","--json"],"Stdout":"}"}
```
</details>

Now, you can make a change to the CR - like changing the `commit` to redeploy to a different commit.  Applying this to the cluster will drive a Pulumi deployment to update the stack.

[operator-sdk]: https://sdk.operatorframework.io/docs/install-operator-sdk/
