FROM pulumi/pulumi:3.227.0-nonroot@sha256:714757d328242f2f72daea421d68d44dc2c9af46944922a0254da0a2f01d8ced

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py