FROM pulumi/pulumi:3.253.0-nonroot@sha256:57696b427ffce33a689fcc536f302433151bf5c6f61f2908e73bf1551ccfb2f9

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py