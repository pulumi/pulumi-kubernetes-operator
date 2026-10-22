FROM pulumi/pulumi:3.272.0-nonroot@sha256:bde6deebdc3150f86cde3ae39052346083ab24ab170f469e68400d133ad9e411

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py