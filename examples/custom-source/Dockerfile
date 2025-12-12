FROM pulumi/pulumi:3.212.0-nonroot@sha256:f7ea8490d2b1b6f5db8f5e71ef73d784ae1189d6a26b9fa17bbd914c17946119

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py