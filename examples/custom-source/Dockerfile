FROM pulumi/pulumi:3.166.0-nonroot@sha256:f9f149225fcddce6eb4680aaece4be8605f310639b63151807a5ccdad5f06592

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py