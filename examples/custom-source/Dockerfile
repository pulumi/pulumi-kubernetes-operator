FROM pulumi/pulumi:3.154.0-nonroot@sha256:363155bb5dfae176e22754c3d5d85a7c0cd00a49af1097e4ce54fe159bce2acf

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py