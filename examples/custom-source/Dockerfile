FROM pulumi/pulumi:3.264.0-nonroot@sha256:87990ce49e923fdfecf5d87f86896d329a8512aa955458ea55108fed92d3ea38

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py