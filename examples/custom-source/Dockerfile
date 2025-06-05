FROM pulumi/pulumi:3.174.0-nonroot@sha256:c855c746e44b8b2de4f3425ffbb4b3f676cb2d9ba3292fbae53b1e9ba71e82d8

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py