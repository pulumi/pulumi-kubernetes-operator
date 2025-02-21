FROM pulumi/pulumi:3.151.0-nonroot@sha256:c12a342814d4460fe7347971ac570ff6c7190ac52c1844c4b73d002924934702

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py