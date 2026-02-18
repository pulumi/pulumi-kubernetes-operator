FROM pulumi/pulumi:3.221.0-nonroot@sha256:6f9094aaaa47f9d83666abf7993e8903546d30c742b79765cd3c9b3fae71ac40

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py