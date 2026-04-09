FROM pulumi/pulumi:3.230.0-nonroot@sha256:ce6375980d71b448df42446f7d0866c1e9bdcfa574b912b1de0a6af7bc74b73f

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py