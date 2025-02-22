FROM pulumi/pulumi:3.152.0-nonroot@sha256:8fe54f5e9008dfaf442b14739264d3395491021b4a75f376a67b3bf47702fd43

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py