FROM pulumi/pulumi:3.234.0-nonroot@sha256:dde963c91e4e66f3dc9b6321970dd280b262219012660fb6ca08b777f2aa4574

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py