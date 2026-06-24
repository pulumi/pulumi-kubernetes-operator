FROM pulumi/pulumi:3.248.0-nonroot@sha256:9a28dd65e9922599404b130b5f50ba75e335d73d1692a00e62c07b13e063aa86

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py