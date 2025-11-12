FROM pulumi/pulumi:3.207.0-nonroot@sha256:313ae9497deebecc5fd5ea71ef3cc9388446fe3655b8319b45e41560e55927e1

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py