FROM pulumi/pulumi:3.188.0-nonroot@sha256:4c1448e36fe0d844123e165f0a5d3f5ec05cc9d38b45dbcf428a0e01937a5989

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py