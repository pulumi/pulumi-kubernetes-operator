FROM pulumi/pulumi:3.155.0-nonroot@sha256:6f0c860004cfad1122da30e9ead5df116fa3ff6bccf4f1b184e1789ce12bdf66

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py