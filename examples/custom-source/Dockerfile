FROM pulumi/pulumi:3.252.0-nonroot@sha256:fe0a7ad7e00450cd78746610c32b277349022cd45bb51bb8e8facebeaf7041d3

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py