FROM pulumi/pulumi:3.255.0-nonroot@sha256:4c4d16e01ec1d7ea377616bff5d960d2542d6c91b0231bdc675962248e60a3c1

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py