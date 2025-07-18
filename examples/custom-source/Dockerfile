FROM pulumi/pulumi:3.184.0-nonroot@sha256:c51b479fef569118209ed4d1436545ce936b79d6ae7ae2a7c040cc7bf6cb487e

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py