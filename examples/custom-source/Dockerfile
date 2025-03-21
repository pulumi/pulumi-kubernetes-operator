FROM pulumi/pulumi:3.157.0-nonroot@sha256:526c826a592313b8081dbdc5a640f3f4e6274feb3b1e05e8aec0ac3f0eab6d3e

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py