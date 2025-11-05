FROM pulumi/pulumi:3.206.0-nonroot@sha256:451ccabd33b674bba5445b770dae8c5ab9c2875f5103778da32d02b15d3bf5e0

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py