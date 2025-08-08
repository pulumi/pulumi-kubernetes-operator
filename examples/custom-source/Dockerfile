FROM pulumi/pulumi:3.188.0-nonroot@sha256:602d6bef87c3ba605ff67d611e7cfcdc0a0c151d89e21d7523a525925b1b11d9

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py