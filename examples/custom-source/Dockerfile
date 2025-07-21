FROM pulumi/pulumi:3.185.0-nonroot@sha256:f4e1fb9a4f8fbe2fea42b7038640ed2d1457dd1483148681bc0ee6b89a0f1d62

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py