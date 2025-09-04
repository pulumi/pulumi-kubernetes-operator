FROM pulumi/pulumi:3.193.0-nonroot@sha256:eca8e5970709bb10e1f1560abcc67df4969ce1e37466f364dcf86e07e65216d4

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py