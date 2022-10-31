# Examples of using Pulumi YAML for inline Programs

The Pulumi Operator can use inline YAML to create Pulumi Programs, using the Program resource. Below are some examples of using Programs of varying complexity.
Before Pulumi Programs can be created, the Operator must first be set up.

## Creating the Operator
// TODO: Use same code as Michael for creating initial Operator deployment?

Install dependencies with npm:
```console
pulumi-yaml$ npm install
```

Run the stack, creating the Operator deployment:
```console
pulumi-yaml$ pulumi up
```

Then create a secret containing your Pulumi access token. This assumes that the PULUMI_ACCESS_TOKEN environment variable is set.
```console
pulumi-yaml$ kubectl create secret generic pulumi-api-secret --from-literal=accessToken="$PULUMI_ACCESS_TOKEN"
```

## Example 1: Creating an nginx deployment with a random number of replicas.

This example generates a random number, which is used to determine the amount of replicas for the nginx deployment.
Because this example only creates resources within the Kubernetes cluster, there is no configuration needed.
You can create this example yourself simply by running:
```console
pulumi-yaml$ kubectl apply -f deployment.yaml
```

## Example 2: Deploying a 'Hello World' Google Cloud Run Container

This example creates a Google Cloud Run service that deploys a basic "Hello World" container that can be publically accessed.
Because this example creates new resources in Google Cloud, authorization must be configured.
This requires a Service Account on Google Cloud. The environment variable `GOOGLE_CREDENTIALS` should be set to the location of the service account credentials file.
```console
pulumi-yaml$ kubectl create secret generic google-credentials --from-file=googleCredentials="$GOOGLE_CREDENTIALS"
```

Once you have configured the program, you can create this example yourself by running:
```console
pulumi-yaml$ kubectl apply -f cloud-run.yaml
```

## Example 3: Creating a static website using AWS S3 and Cloudfront

This example creates an S3 bucket, adds a HTML file to the bucket, and then exposes that file to the internet via Cloudfront.
Because this example creates new resources in AWS, there is some configuration required.
Once you have configured the program, you can create this example yourself by running:
```console
pulumi-yaml$ kubectl apply -f s3-website.yaml
```