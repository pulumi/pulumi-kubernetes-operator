FROM pulumi/pulumi:3.225.0-nonroot@sha256:56ca232c173460291f57510772481dc9d65e5c2d549eaba910f39f261bfd9c76

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py