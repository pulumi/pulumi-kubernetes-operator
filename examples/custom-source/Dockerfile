FROM pulumi/pulumi:3.254.0-nonroot@sha256:f2ebc75daa74187b66d34c8ee9eafe7f01a12357b906179ab2403a3987b9fd56

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py