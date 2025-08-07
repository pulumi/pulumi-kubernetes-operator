FROM pulumi/pulumi:3.188.0-nonroot@sha256:5f7231a98b4dca9f92770bb1ea85311bd1912b5cdece0cc5b6b23a3750879a70

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py