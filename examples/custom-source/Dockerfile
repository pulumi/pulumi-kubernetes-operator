FROM pulumi/pulumi:3.216.0-nonroot@sha256:6b52f829be6d94eca8527d7875cd5c245091cec8ffb8d2afcb427dff048ecb59

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py