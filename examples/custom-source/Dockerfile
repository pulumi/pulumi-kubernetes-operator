FROM pulumi/pulumi:3.153.0-nonroot@sha256:dca3dd9218b629c3342ecaffe07c6ff363c8b00eb0916383ac34b463048e868a

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py