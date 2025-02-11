FROM pulumi/pulumi:3.149.0-nonroot@sha256:e24eaa86fbadf63a41664c11fee4512f4902dcbf3055ad1f23d2155cd3bcff83

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py