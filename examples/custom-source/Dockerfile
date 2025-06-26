FROM pulumi/pulumi:3.178.0-nonroot@sha256:5c05b9a8c1a365518ac9c73ccb849ad06bfcf8e89e7bcdc318b6b439b9dd05ef

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py