FROM pulumi/pulumi:3.165.0-nonroot@sha256:3e9f8e15d1b45b45b1f798330a4719d37c0cf4769a03a5ccb7678ccf44144612

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py