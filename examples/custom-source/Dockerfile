FROM pulumi/pulumi:3.250.0-nonroot@sha256:785f4e4b0d0d04b4af3025d8b8cf251e9f153000e09adcf2daf8b3cf074aa81b

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py