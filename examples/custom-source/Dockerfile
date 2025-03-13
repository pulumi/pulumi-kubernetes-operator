FROM pulumi/pulumi:3.156.0-nonroot@sha256:4b61ea217112abe72123db37250d286ed62802a51fe2d36a65f494673d130e43

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py