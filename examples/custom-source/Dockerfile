FROM pulumi/pulumi:3.274.0-nonroot@sha256:2ef11ca679b4995cbee09b351c1303e52bbe7b4cd952ab353f21b8fbd442b9b2

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py