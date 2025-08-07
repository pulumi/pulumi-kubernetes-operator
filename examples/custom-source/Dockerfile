FROM pulumi/pulumi:3.188.0-nonroot@sha256:682491116ed594f394e51279bbe16f3c75f2fd51cc2c06fa7d4276b05a17a509

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py