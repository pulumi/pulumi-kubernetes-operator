FROM pulumi/pulumi:3.256.0-nonroot@sha256:c1e7729779d54594fbcdd7714999c9d0d28cb4306bfcb4eeb1029af063698a83

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py