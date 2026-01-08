FROM pulumi/pulumi:3.215.0-nonroot@sha256:08a07e738fc92d81da47128ed980a141cba58e24a0b9c896a3cb87a31e24809c

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py