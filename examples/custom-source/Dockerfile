FROM pulumi/pulumi:3.153.1-nonroot@sha256:c28c60b24761a0e222b7ab9d6e3e7809acbe02439b4921ff71ceb994893e90d0

# Clone the git repository
RUN git clone --depth 1 https://github.com/pulumi/examples.git /home/pulumi/examples

# Preinstall the plugins for the 'stack-readme-py' program
RUN pulumi install -C /home/pulumi/examples/stack-readme-py