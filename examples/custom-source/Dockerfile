FROM pulumi/pulumi:3.278.0-nonroot@sha256:6f68c6a94a1f9f42941c031366a9d3c6b790bd92362ae2bc693fa801c3dc98e8

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py