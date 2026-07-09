FROM pulumi/pulumi:3.251.0-nonroot@sha256:373afa48d3a6228aa54089275d7513d0e94f91139d29d27275d0539800fdc3dd

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py