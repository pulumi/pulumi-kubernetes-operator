FROM pulumi/pulumi:3.213.0-nonroot@sha256:e54df2adf15716b8f91791319d339b3e58ba60be42e0474b5b0375e864fe3205

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py