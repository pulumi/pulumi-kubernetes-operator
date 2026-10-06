FROM pulumi/pulumi:3.267.0-nonroot@sha256:3be05b9aec48341b09e1722ca53a1df0fa6e88f7f022cb5d2b0e0a1c463aa9f7

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py