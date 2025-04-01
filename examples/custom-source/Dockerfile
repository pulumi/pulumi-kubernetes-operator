FROM pulumi/pulumi:3.160.0-nonroot@sha256:decabfa74156b59240cb1c39a3e86b5aa5c65512e15bd32d4ad6cba480ebd066

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py