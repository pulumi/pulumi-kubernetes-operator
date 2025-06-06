FROM pulumi/pulumi:3.175.0-nonroot@sha256:a2ca650674c2a08b586e56a8e514c73e28f0761c5c0c4a1e3acdee49feec082a

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py