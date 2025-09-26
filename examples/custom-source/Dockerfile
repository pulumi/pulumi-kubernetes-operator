FROM pulumi/pulumi:3.198.0-nonroot@sha256:4e580aefa911a7ef187cb68cae5c7d5e74765929239d6fba018288c13515b971

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py