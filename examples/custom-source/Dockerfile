FROM pulumi/pulumi:3.268.0-nonroot@sha256:4ad97634747ebe3335e5b60f7b102f7e17862ed6a0e6ee6fdc9e0cf500221d0c

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py