FROM pulumi/pulumi:3.269.0-nonroot@sha256:3235a687f605bb0a1c6e3a75515e0e088f7aacf89899a6abca4220e4e2fd3ec5

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py