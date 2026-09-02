FROM pulumi/pulumi:3.261.0-nonroot@sha256:56adc4ba076bb3f4a4d72083adb1dbc5ad1b20bb1b126ccbf27b304d19d04dcb

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py