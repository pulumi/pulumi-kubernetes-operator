FROM pulumi/pulumi:3.169.0-nonroot@sha256:cb8c857ee14897a46bf6552dee2d8ac6506266cab975e1ad12cf0a0d278dbebe

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py