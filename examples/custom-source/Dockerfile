FROM pulumi/pulumi:3.210.0-nonroot@sha256:8ee9bbcc8f2cbbe412dd4533511b56b8ce54d2f8c1bfeac1dbae4c9af973d1f3

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py