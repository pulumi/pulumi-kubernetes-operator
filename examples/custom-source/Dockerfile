FROM pulumi/pulumi:3.217.1-nonroot@sha256:62cdf3a2dc95cbec99f88d9f02d1c0e3a4c04be115af9916b098b2ed33e25d1b

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py