FROM pulumi/pulumi:3.258.0-nonroot@sha256:76728fd277e72c23c2e5d7014128a2603ab002d1c33cbbe1fdbc3c8425c328fb

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py