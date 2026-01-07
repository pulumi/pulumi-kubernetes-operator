FROM pulumi/pulumi:3.215.0-nonroot@sha256:d430069da8a1cd038669d433e7d361f5815c4d8a0773957efade8b7f9f5438b1

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py