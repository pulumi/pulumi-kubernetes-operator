FROM pulumi/pulumi:3.202.0-nonroot@sha256:87b000ac99932e5a75bd35192a2ac6bcf79b2952b6041c93c01b1bf03bb86937

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py