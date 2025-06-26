FROM pulumi/pulumi:3.180.0-nonroot@sha256:c86e396f6dc34022be93f127c4ca5d93d944d8082db371543121f09af1dd9a59

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py