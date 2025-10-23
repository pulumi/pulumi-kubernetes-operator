FROM pulumi/pulumi:3.204.0-nonroot@sha256:f1017bde3092fc8a6d117f4db199f045a17f4e2eb1f69715ccd737766d27a3d1

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py