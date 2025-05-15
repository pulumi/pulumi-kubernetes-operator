FROM pulumi/pulumi:3.170.0-nonroot@sha256:c35cb39a1bb921f156aff2dd2d92a82f4a650a41727be597ac1ecd3eb1e69ca3

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py