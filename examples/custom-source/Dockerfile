FROM pulumi/pulumi:3.220.0-nonroot@sha256:3d4f8f8f9645058ccb2343d50ff6de7947c8914792c8737f03957f5c90161534

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py