FROM pulumi/pulumi:3.260.0-nonroot@sha256:3f314ba30e39ee4080f234f025cb8e09a5aef4a5f4c64a953cf2232f2b2bd1c7

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py