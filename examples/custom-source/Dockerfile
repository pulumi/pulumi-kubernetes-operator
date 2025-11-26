FROM pulumi/pulumi:3.209.0-nonroot@sha256:f59220f4c0006527549e6fb1fe8aa3db9bc0c0960dbd01b78505f2d3b6f1ed78

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py