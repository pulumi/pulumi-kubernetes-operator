FROM pulumi/pulumi:3.257.0-nonroot@sha256:b0377c031c065bbece36a8e3fac1e431b7fdeccf6018c4d5df1d642ae9e1dff8

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py