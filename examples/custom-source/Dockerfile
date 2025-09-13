FROM pulumi/pulumi:3.196.0-nonroot@sha256:4f511007ff79856f4f5ac956092375ccf43a640a0c9ba165f96b54017eab5f45

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py