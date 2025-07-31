FROM pulumi/pulumi:3.187.0-nonroot@sha256:fd6572241e1d2fc7a6a854cd73511fa232eefb88e27b91f61ec2de7f985f6196

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py