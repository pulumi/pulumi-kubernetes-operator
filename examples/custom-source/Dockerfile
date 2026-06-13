FROM pulumi/pulumi:3.246.0-nonroot@sha256:35818c88fefc74e289131c238dfe4e6e794c35ebfea7b988778abdcfbd508c8f

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py