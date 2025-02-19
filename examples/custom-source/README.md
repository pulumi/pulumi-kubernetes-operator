Demonstrates how to use your own program source rather than relying on built-in support for Git and Flux.

To use your own source, you have two options:

1. Use an init container to copy Pulumi project files to `/share/workspace`, or
2. Bake your Pulumi project files into the workspace image.

This example explores the second option. The procedure is:

1. Build a Docker image containing your project files. See the example Dockerfile that clones Pulumi's repository of examples. It also runs `pulumi install` to pre-install your program's dependencies.
2. In the `Stack` specification, customize the workspace template to point to a Pulumi project within the cloned repository, using `.spec.local.dir`.
3. Deploy the stack. The system automatically links to the local directory.